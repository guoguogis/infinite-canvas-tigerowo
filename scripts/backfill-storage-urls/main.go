// 一次性维护脚本：把数据库里历史遗留的裸对象地址 /api/files/<id>/content
// 批量补上签名参数，使其继续可通过下载签名校验。
//
// 用法（默认只统计，不写库）：
//
//	go run ./scripts/backfill-storage-urls
//	go run ./scripts/backfill-storage-urls -apply
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"github.com/tigerowo/infinite-canvas/service"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var contentURLPattern = regexp.MustCompile(`/api/files/([0-9A-Za-z_-]+)/content(?:\?([^"'\s\\]*))?`)

var targets = []any{
	&model.User{},
	&model.CreditLog{},
	&model.Prompt{},
	&model.PromptSource{},
	&model.AgentSkill{},
	&model.AgentSkillFile{},
	&model.Asset{},
	&model.Setting{},
	&model.CreativeWorkflow{},
	&model.UserConfig{},
	&model.StorageObject{},
	&model.VideoTask{},
	&model.VideoGenerationLog{},
	&model.ImageGenerationLog{},
	&model.CanvasImageTask{},
	&model.CanvasAudioTask{},
	&model.CanvasProject{},
	&model.ComfyBridge{},
	&model.ComfyBridgeRequest{},
}

func main() {
	apply := flag.Bool("apply", false, "真正写库；默认只统计将要修改的行数")
	verify := flag.Bool("verify", false, "只读扫描全部表，列出仍含未签名对象地址的表与字段")
	flag.Parse()

	if err := config.Load(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := repository.DB()
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	if *verify {
		verifyAll(db)
		return
	}

	totalRows, totalFields := 0, 0
	for _, target := range targets {
		rows, fields, err := processTable(db, target, *apply)
		if err != nil {
			log.Fatalf("处理 %s 失败: %v", schemaOf(db, target).Table, err)
		}
		if rows > 0 {
			fmt.Printf("%-26s 行 %-6d 字段 %d\n", schemaOf(db, target).Table, rows, fields)
		}
		totalRows += rows
		totalFields += fields
	}
	mode := "待回填"
	if *apply {
		mode = "已回填"
	}
	fmt.Printf("\n%s：%d 行 / %d 个字段\n", mode, totalRows, totalFields)
	if !*apply && totalRows > 0 {
		fmt.Println("加上 -apply 参数才会真正写库。")
	}
}

func processTable(db *gorm.DB, target any, apply bool) (int, int, error) {
	tableSchema := schemaOf(db, target)
	pkNames := make([]string, 0, len(tableSchema.PrimaryFields))
	for _, field := range tableSchema.PrimaryFields {
		pkNames = append(pkNames, field.DBName)
	}
	if len(pkNames) == 0 {
		return 0, 0, fmt.Errorf("缺少主键")
	}
	columnTypes, err := db.Migrator().ColumnTypes(target)
	if err != nil {
		return 0, 0, err
	}
	rows, err := loadRows(db, tableSchema.Table)
	if err != nil {
		return 0, 0, err
	}

	changedRows, changedFields := 0, 0
	for _, row := range rows {
		updates := map[string]any{}
		for _, columnType := range columnTypes {
			name := columnType.Name()
			if isPrimaryKey(name, pkNames) || !isTextColumn(columnType.DatabaseTypeName()) {
				continue
			}
			value, ok := row[name].(string)
			if !ok {
				continue
			}
			if next := rewriteContentURLs(value); next != value {
				updates[name] = next
			}
		}
		if len(updates) == 0 {
			continue
		}
		changedRows++
		changedFields += len(updates)
		if !apply {
			continue
		}
		where := map[string]any{}
		for _, name := range pkNames {
			where[name] = row[name]
		}
		if err := db.Table(tableSchema.Table).Where(where).Updates(updates).Error; err != nil {
			return 0, 0, err
		}
	}
	return changedRows, changedFields, nil
}

// verifyAll 只读扫描数据库中的全部表，列出仍含未签名对象地址的表与字段。
func verifyAll(db *gorm.DB) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		log.Fatalf("读取表清单失败: %v", err)
	}
	found := 0
	for _, table := range tables {
		columnTypes, err := db.Migrator().ColumnTypes(table)
		if err != nil {
			log.Fatalf("读取 %s 字段失败: %v", table, err)
		}
		textColumns := make([]string, 0, len(columnTypes))
		for _, columnType := range columnTypes {
			if isTextColumn(columnType.DatabaseTypeName()) {
				textColumns = append(textColumns, columnType.Name())
			}
		}
		if len(textColumns) == 0 {
			continue
		}
		rows, err := loadRows(db, table)
		if err != nil {
			log.Fatalf("读取 %s 数据失败: %v", table, err)
		}
		for _, name := range textColumns {
			rowsWithBare, totalBare := 0, 0
			for _, row := range rows {
				value, ok := row[name].(string)
				if !ok {
					continue
				}
				count := countUnsignedContentURLs(value)
				if count > 0 {
					rowsWithBare++
					totalBare += count
				}
			}
			if rowsWithBare > 0 {
				found++
				fmt.Printf("%-26s %-22s 行 %-5d 处 %d\n", table, name, rowsWithBare, totalBare)
			}
		}
	}
	if found == 0 {
		fmt.Println("全部表的对象地址都已带签名。")
	}
}

func countUnsignedContentURLs(text string) int {
	if !strings.Contains(text, "/api/files/") {
		return 0
	}
	count := 0
	for _, match := range contentURLPattern.FindAllStringSubmatch(text, -1) {
		query := ""
		if len(match) > 2 {
			query = match[2]
		}
		if !hasSignatureParam(query) {
			count++
		}
	}
	return count
}

func hasSignatureParam(query string) bool {
	for _, pair := range strings.Split(query, "&") {
		if key, _, _ := strings.Cut(pair, "="); key == "s" {
			return true
		}
	}
	return false
}

// loadRows 直接扫描原始结果，绕开模型的字段序列化器（例如 Assets.Tags 这类 JSON 字段）。
func loadRows(db *gorm.DB, table string) ([]map[string]any, error) {
	sqlRows, err := db.Table(table).Select("*").Rows()
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	columns, err := sqlRows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for sqlRows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := sqlRows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for index, name := range columns {
			if raw, ok := values[index].([]byte); ok {
				row[name] = string(raw)
			} else {
				row[name] = values[index]
			}
		}
		result = append(result, row)
	}
	if err := sqlRows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return result, nil
}

func rewriteContentURLs(text string) string {
	if !strings.Contains(text, "/api/files/") {
		return text
	}
	return contentURLPattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := contentURLPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		query := url.Values{}
		if len(groups) > 2 {
			for _, pair := range strings.Split(groups[2], "&") {
				if pair == "" {
					continue
				}
				key, value, _ := strings.Cut(pair, "=")
				if key == "s" {
					continue
				}
				query.Set(key, value)
			}
		}
		signature := signatureOf(groups[1])
		if signature == "" {
			return match
		}
		query.Set("s", signature)
		return "/api/files/" + groups[1] + "/content?" + query.Encode()
	})
}

// signatureOf 复用服务端的签名实现，避免脚本与后端算法不一致。
func signatureOf(id string) string {
	parsed, err := url.Parse(service.StorageObjectContentURL(id))
	if err != nil {
		return ""
	}
	return parsed.Query().Get("s")
}

func schemaOf(db *gorm.DB, target any) *schema.Schema {
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(target); err != nil {
		log.Fatalf("解析模型失败: %v", err)
	}
	return statement.Schema
}

func isPrimaryKey(name string, pkNames []string) bool {
	for _, pk := range pkNames {
		if pk == name {
			return true
		}
	}
	return false
}

func isTextColumn(databaseType string) bool {
	upper := strings.ToUpper(databaseType)
	return strings.Contains(upper, "TEXT") || strings.Contains(upper, "CHAR") || strings.Contains(upper, "CLOB")
}
