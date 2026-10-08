package service

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

// promptSourceRegistryBase 是内置来源使用的 JSON 地址前缀。
const promptSourceRegistryBase = "https://raw.githubusercontent.com/yukkcat/image-prompts/main/dist/sources"

// promptSourceRegistryHomepage 是注册表来源的主页。
const promptSourceRegistryHomepage = "https://github.com/yukkcat/image-prompts"

// YouMind skill 仓库：一个 manifest 索引 + 多个分类文件，11 个分类共 1.5 万条左右。
const youmindAIImagePromptsID = "youmind-ai-image-prompts"
const youmindAIImagePromptsName = "YouMind AI Image Prompts"
const youmindAIImagePromptsHome = "https://github.com/YouMind-OpenLab/ai-image-prompts-skill"
const youmindAIImagePromptsManifest = "https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references/manifest.json"

// defaultPromptSources 返回内置提示词来源。与项目已有分类重合的默认禁用，避免重复导入。
func defaultPromptSources() []model.PromptSource {
	presets := []struct {
		id      string
		name    string
		enabled bool
	}{
		// 视频来源：数据由内置生成脚本产出，同步时直接读取，无需外网。
		{id: ATLAS_MINIMAX_H3_PROMPTS_ID, name: ATLAS_MINIMAX_H3_PROMPTS_NAME, enabled: true},
		{id: SEEDANCE_2_PROMPTS_ID, name: SEEDANCE_2_PROMPTS_NAME, enabled: true},
		{id: LANSHU_VIDEO_PROMPTS_ID, name: LANSHU_VIDEO_PROMPTS_NAME, enabled: true},
		// 图像来源：从注册表 JSON 拉取。
		{id: "banana-prompt-quicker", name: "Banana Prompt Quicker", enabled: true},
		{id: "freestylefly-gpt-image-2", name: "Freestylefly GPT Image 2", enabled: true},
		{id: "awesome-gpt-image", name: "Awesome GPT Image", enabled: false},
		{id: "awesome-gpt4o-image-prompts", name: "Awesome GPT-4o Image Prompts", enabled: false},
		// 这两个仓库已由「分类同步」路径每天自动抓取，来源方式默认禁用避免重复导入。
		{id: "youmind-gpt-image-2", name: "YouMind GPT Image 2", enabled: false},
		{id: "youmind-nano-banana-pro", name: "YouMind Nano Banana Pro", enabled: false},
		{id: "davidwu-gpt-image2-prompts", name: "DavidWu GPT Image 2", enabled: false},
	}
	items := make([]model.PromptSource, 0, len(presets)+1)
	builtinHomes := map[string]string{
		ATLAS_MINIMAX_H3_PROMPTS_ID: ATLAS_MINIMAX_H3_PROMPTS_HOME,
		SEEDANCE_2_PROMPTS_ID:  SEEDANCE_2_PROMPTS_HOME,
		LANSHU_VIDEO_PROMPTS_ID:    LANSHU_VIDEO_PROMPTS_HOME,
	}
	for _, preset := range presets {
		if home, ok := builtinHomes[preset.id]; ok {
			items = append(items, model.PromptSource{ID: preset.id, Name: preset.name, Homepage: home, Category: preset.id, Kind: model.PromptSourceKindBuiltin, Enabled: preset.enabled, BuiltIn: true})
			continue
		}
		items = append(items, model.PromptSource{
			ID:       preset.id,
			Name:     preset.name,
			URL:      promptSourceRegistryBase + "/" + preset.id + ".json",
			Homepage: promptSourceRegistryHomepage,
			Category: preset.id,
			Kind:     model.PromptSourceKindRemote,
			Enabled:  preset.enabled,
			BuiltIn:  true,
		})
	}
	items = append(items, model.PromptSource{
		ID:       youmindAIImagePromptsID,
		Name:     youmindAIImagePromptsName,
		URL:      youmindAIImagePromptsManifest,
		Homepage: youmindAIImagePromptsHome,
		Category: youmindAIImagePromptsID,
		Kind:     model.PromptSourceKindManifest,
		Enabled:  true,
		BuiltIn:  true,
	})
	return items
}

// EnsureDefaultPromptSources 写入缺失的内置来源，并把历史数据补齐类型字段。
func EnsureDefaultPromptSources() error {
	existing, err := repository.ListPromptSources()
	if err != nil {
		return err
	}
	saved := map[string]model.PromptSource{}
	for _, item := range existing {
		saved[item.ID] = item
	}
	for _, preset := range defaultPromptSources() {
		current, ok := saved[preset.ID]
		if !ok {
			if _, err := repository.SavePromptSource(preset); err != nil {
				return err
			}
			continue
		}
		changed := false
		if current.Kind == "" {
			current.Kind = preset.Kind
			changed = true
		}
		if current.Homepage == "" && preset.Homepage != "" {
			current.Homepage = preset.Homepage
			changed = true
		}
		if changed {
			if _, err := repository.SavePromptSource(current); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListPromptSources 返回全部提示词来源，并确保内置来源已初始化。
func ListPromptSources() ([]model.PromptSource, error) {
	if err := EnsureDefaultPromptSources(); err != nil {
		return nil, err
	}
	return repository.ListPromptSources()
}

// PromptSourceInput 是新增或编辑来源的入参。
type PromptSourceInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Homepage string `json:"homepage"`
	Enabled  *bool  `json:"enabled"`
}

// SavePromptSource 新增或更新一个提示词来源。
func SavePromptSource(input PromptSourceInput) ([]model.PromptSource, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("请输入来源名称")
	}
	rawURL, err := validatePromptSourceURL(input.URL)
	if err != nil {
		return nil, err
	}
	homepage := strings.TrimSpace(input.Homepage)

	source := model.PromptSource{}
	if id := strings.TrimSpace(input.ID); id != "" {
		saved, ok, err := repository.FindPromptSource(id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("提示词来源不存在")
		}
		source = saved
	} else {
		source.ID = newPromptSourceID(name)
		source.Enabled = true
		source.Kind = model.PromptSourceKindRemote
	}
	if source.BuiltIn && source.URL != rawURL {
		return nil, errors.New("内置来源的地址不可修改")
	}
	source.Name = name
	source.URL = rawURL
	source.Homepage = homepage
	if source.Category == "" {
		source.Category = source.ID
	}
	if input.Enabled != nil {
		source.Enabled = *input.Enabled
	}
	if _, err := repository.SavePromptSource(source); err != nil {
		return nil, err
	}
	return ListPromptSources()
}

// DeletePromptSource 删除非内置的提示词来源。
func DeletePromptSource(id string) ([]model.PromptSource, error) {
	source, ok, err := repository.FindPromptSource(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("提示词来源不存在")
	}
	if source.BuiltIn {
		return nil, errors.New("内置来源不能删除")
	}
	if err := repository.DeletePromptSource(source.ID); err != nil {
		return nil, err
	}
	return ListPromptSources()
}

// SyncPromptSource 拉取并替换单个来源的提示词。
func SyncPromptSource(id string) ([]model.PromptSource, error) {
	source, ok, err := repository.FindPromptSource(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("提示词来源不存在")
	}
	if err := syncPromptSource(source); err != nil {
		// 把失败原因记在来源上，管理页才能显示「同步失败」。
		source.LastError = err.Error()
		_, _ = repository.SavePromptSource(source)
		return nil, err
	}
	return ListPromptSources()
}

// SyncAllPromptSources 依次同步全部启用的来源，返回各来源的失败信息。
func SyncAllPromptSources() ([]model.PromptSource, map[string]string, error) {
	items, err := ListPromptSources()
	if err != nil {
		return nil, nil, err
	}
	failures := map[string]string{}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if err := syncPromptSource(item); err != nil {
			failures[item.Name] = err.Error()
		}
	}
	synced, err := ListPromptSources()
	return synced, failures, err
}

func syncPromptSource(source model.PromptSource) error {
	if source.Kind == model.PromptSourceKindManifest {
		if err := syncManifestPromptSource(source); err != nil {
			return err
		}
	} else {
		items, err := promptSourceItems(source)
		if err != nil {
			return err
		}
		if err := repository.ReplacePromptSourcePrompts(source.ID, items); err != nil {
			return err
		}
	}
	source.LastSyncAt = time.Now().Format(time.RFC3339)
	source.LastError = ""
	_, err := repository.SavePromptSource(source)
	return err
}

// promptSourceItems 取出来源的全部提示词（内置数据或单个远程 JSON）。
func promptSourceItems(source model.PromptSource) ([]model.Prompt, error) {
	if source.Kind == model.PromptSourceKindBuiltin {
		return builtinPromptSourceItems(source)
	}
	raw, err := fetchPromptSourceText(source.URL)
	if err != nil {
		return nil, err
	}
	return parsePromptSourceItems(raw, source, time.Now().Format(time.RFC3339))
}

// promptSourceManifest 是清单型来源的索引结构。
type promptSourceManifest struct {
	Categories []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		File  string `json:"file"`
	} `json:"categories"`
}

// syncManifestPromptSource 按清单逐个分类拉取并写入，接口仍是「替换整个来源」。
// 逐分类写入可以避免把上万条全部堆在内存里，也把单次数据库写入量控制住。
func syncManifestPromptSource(source model.PromptSource) error {
	raw, err := fetchPromptSourceText(source.URL)
	if err != nil {
		return err
	}
	var manifest promptSourceManifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return errors.New("来源清单 JSON 格式不正确")
	}
	if len(manifest.Categories) == 0 {
		return errors.New("来源清单没有分类")
	}
	if err := repository.DeletePromptSourcePrompts(source.ID); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	total := 0
	for _, category := range manifest.Categories {
		file := strings.TrimSpace(category.File)
		if file == "" {
			continue
		}
		rawCategory, err := fetchPromptSourceText(resolvePromptSourceURL(source.URL, file))
		if err != nil {
			return fmt.Errorf("已写入 %d 条；拉取 %s 失败：%w", total, file, err)
		}
		slug := slugPromptSourceID(firstNonEmptyString(category.Slug, category.Title))
		parsed, err := parsePromptSourceItemsIn(rawCategory, source, slug+"-", func(int) string { return now })
		if err != nil {
			return fmt.Errorf("已写入 %d 条；解析 %s 失败：%w", total, file, err)
		}
		for index := range parsed {
			parsed[index].Tags = appendPromptSourceTag(parsed[index].Tags, strings.TrimSpace(category.Title))
		}
		if err := repository.AppendPromptSourcePrompts(source.ID, parsed); err != nil {
			return fmt.Errorf("已写入 %d 条；写入 %s 失败：%w", total, file, err)
		}
		total += len(parsed)
	}
	if total == 0 {
		return errors.New("来源没有解析到有效提示词")
	}
	return nil
}

func appendPromptSourceTag(tags []string, tag string) []string {
	if tag == "" {
		return tags
	}
	for _, item := range tags {
		if item == tag {
			return tags
		}
	}
	return append(tags, tag)
}

// builtinPromptSourceItems 取用程序内置的提示词数据。
func builtinPromptSourceItems(source model.PromptSource) ([]model.Prompt, error) {
	var encoded string
	switch source.ID {
	case ATLAS_MINIMAX_H3_PROMPTS_ID:
		encoded = strings.Join(atlasMinimaxH3PromptsPromptSourceData, "")
	case SEEDANCE_2_PROMPTS_ID:
		encoded = strings.Join(seedance2PromptsPromptSourceData, "")
	case LANSHU_VIDEO_PROMPTS_ID:
		encoded = strings.Join(lanshuVideoPromptsPromptSourceData, "")
	default:
		return nil, errors.New("该来源没有内置数据")
	}
	raw, err := decompressPromptSourceData(encoded)
	if err != nil {
		return nil, err
	}
	// 内置数据没有时间字段，按顺序递减生成时间戳，保证列表顺序稳定。
	base := time.Now().Truncate(time.Minute)
	return parsePromptSourceItemsWithTime(raw, source, func(index int) string {
		return base.Add(-time.Duration(index) * time.Second).Format(time.RFC3339)
	})
}

func decompressPromptSourceData(encoded string) (string, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("内置提示词数据解码失败")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", errors.New("内置提示词数据解压失败")
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", errors.New("内置提示词数据读取失败")
	}
	return string(raw), nil
}

// promptSourceFetchTimeout 是拉取来源内容（含清单里的分类文件）的单次超时。
// 清单型来源可能有几十 MB，慢速网络下 60 秒不够用，这里放宽到 20 分钟。
const promptSourceFetchTimeout = 20 * time.Minute

// promptSourceFetchAttempts 是单次拉取的尝试次数，公网源偶发失败时自动重试。
const promptSourceFetchAttempts = 3

// promptSourceHTTPClient 放宽默认传输层超时：默认 TLS 握手只有 10 秒，
// 跨境或慢速网络下经常在握手阶段就超时失败。
var promptSourceHTTPClient = &http.Client{
	Timeout: promptSourceFetchTimeout,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	},
}

func fetchPromptSourceText(address string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= promptSourceFetchAttempts; attempt++ {
		body, err := fetchPromptSourceTextOnce(address)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if attempt < promptSourceFetchAttempts {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	return "", lastErr
}

func fetchPromptSourceTextOnce(address string) (string, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return "", errors.New("来源地址无效")
	}
	response, err := promptSourceHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("拉取来源内容失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("拉取来源内容失败：HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return "", fmt.Errorf("读取来源内容失败：%w", err)
	}
	return string(body), nil
}

// promptSourceItem 兼容注册表与常见提示词 JSON 的字段命名。
// id 可能是字符串或数字，sourceMedia 是 skill 类仓库使用的图片字段。
type promptSourceItem struct {
	ID                 any      `json:"id"`
	Title              string   `json:"title"`
	Name               string   `json:"name"`
	Prompt             string   `json:"prompt"`
	Content            string   `json:"content"`
	Description        string   `json:"description"`
	CoverURL           string   `json:"coverUrl"`
	Cover              string   `json:"cover"`
	Image              string   `json:"image"`
	ReferenceImageURLs []string `json:"referenceImageUrls"`
	SourceMedia        []string `json:"sourceMedia"`
	VideoURLs          []string `json:"videoUrls"`
	VideoURL           string   `json:"videoUrl"`
	Tags               []string `json:"tags"`
	Preview            string   `json:"preview"`
	Author             string   `json:"author"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

func parsePromptSourceItems(raw string, source model.PromptSource, now string) ([]model.Prompt, error) {
	return parsePromptSourceItemsIn(raw, source, "", func(int) string { return now })
}

// parsePromptSourceItemsWithTime 解析提示词数组，时间戳由调用方按序号生成。
func parsePromptSourceItemsWithTime(raw string, source model.PromptSource, timestampAt func(index int) string) ([]model.Prompt, error) {
	return parsePromptSourceItemsIn(raw, source, "", timestampAt)
}

// parsePromptSourceItemsIn 解析提示词数组，时间戳由调用方按序号生成。
// idPrefix 用于清单型来源按分类隔离自增编号，避免不同分类文件之间 ID 冲突。
func parsePromptSourceItemsIn(raw string, source model.PromptSource, idPrefix string, timestampAt func(index int) string) ([]model.Prompt, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("来源返回内容为空")
	}
	values := []promptSourceItem{}
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, errors.New("来源 JSON 格式不正确，应为提示词数组")
	}
	items := make([]model.Prompt, 0, len(values))
	seen := map[string]bool{}
	for index, value := range values {
		title := strings.TrimSpace(firstNonEmptyString(value.Title, value.Name))
		prompt := strings.TrimSpace(firstNonEmptyString(value.Prompt, value.Content))
		if title == "" || prompt == "" {
			continue
		}
		id := strings.TrimSpace(promptSourceIDString(value.ID))
		switch {
		case id == "":
			id = fmt.Sprintf("%s%s-%04d", idPrefix, source.ID, index+1)
		case !strings.HasPrefix(id, source.ID):
			id = source.ID + "-" + idPrefix + id
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		createdAt := firstNonEmptyString(strings.TrimSpace(value.CreatedAt), timestampAt(index))
		items = append(items, model.Prompt{
			ID:        id,
			Title:     title,
			CoverURL:  resolvePromptSourceURL(source.URL, firstNonEmptyString(value.CoverURL, value.Cover, value.Image, firstString(value.ReferenceImageURLs), firstString(value.SourceMedia))),
			Prompt:    prompt,
			Tags:      normalizePromptSourceTags(value.Tags, value.Author),
			Category:  source.Category,
			SourceID:  source.ID,
			VideoURL:  strings.TrimSpace(firstNonEmptyString(value.VideoURL, firstString(value.VideoURLs))),
			Preview:   strings.TrimSpace(firstNonEmptyString(value.Description, value.Preview)),
			CreatedAt: createdAt,
			UpdatedAt: firstNonEmptyString(strings.TrimSpace(value.UpdatedAt), createdAt),
		})
	}
	if len(items) == 0 {
		return nil, errors.New("来源没有解析到有效提示词")
	}
	return items, nil
}

// promptSourceIDString 兼容字符串与数字两种 ID。
func promptSourceIDString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func normalizePromptSourceTags(tags []string, author string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if author = strings.TrimSpace(author); author != "" && !seen[author] {
		result = append(result, author)
	}
	return result
}

// resolvePromptSourceURL 把来源里的相对地址（图片、分类文件）解析成绝对地址。
func resolvePromptSourceURL(baseURL string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		return value
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return base.Scheme + "://" + base.Host + value
	}
	base.Path = path.Join(path.Dir(base.Path), value)
	return base.String()
}

func validatePromptSourceURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("请输入 JSON 地址")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("请输入有效的 JSON 地址")
	}
	return trimmed, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// newPromptSourceID 依据名称生成来源 ID，重名时追加序号。
func newPromptSourceID(name string) string {
	base := slugPromptSourceID(name)
	if base == "" {
		base = "source"
	}
	existing, _ := repository.ListPromptSources()
	used := map[string]bool{}
	for _, item := range existing {
		used[item.ID] = true
	}
	if !used[base] {
		return base
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", base, index)
		if !used[candidate] {
			return candidate
		}
	}
}

func slugPromptSourceID(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		case char >= 0x4e00 && char <= 0x9fff:
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
