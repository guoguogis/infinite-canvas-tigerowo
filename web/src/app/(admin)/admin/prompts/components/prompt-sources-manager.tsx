"use client";

import { CopyOutlined, DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { App as AntdApp, Button, Drawer, Form, Input, Modal, Space, Switch, Table, Tag, Typography } from "antd";
import { useEffect, useState } from "react";

import { useCopyText } from "@/hooks/use-copy-text";
import type { AdminPromptSource, AdminPromptSourceInput } from "@/services/api/admin";

function isHttpUrl(value: string) {
    try {
        const url = new URL(value);
        return url.protocol === "http:" || url.protocol === "https:";
    } catch {
        return false;
    }
}

export function PromptSourcesManager({
    open,
    sources,
    loading,
    syncing,
    onClose,
    onSave,
    onDelete,
    onSync,
    onSyncAll,
}: {
    open: boolean;
    sources: AdminPromptSource[];
    loading: boolean;
    syncing: boolean;
    onClose: () => void;
    onSave: (input: AdminPromptSourceInput) => Promise<unknown>;
    onDelete: (id: string) => Promise<unknown>;
    onSync: (id: string) => Promise<unknown>;
    onSyncAll: () => Promise<unknown>;
}) {
    const { message } = AntdApp.useApp();
    const copyText = useCopyText();
    const [draft, setDraft] = useState<AdminPromptSourceInput | null>(null);
    const [form] = Form.useForm<AdminPromptSourceInput>();

    useEffect(() => {
        if (draft) form.setFieldsValue({ name: draft.name, url: draft.url, homepage: draft.homepage, enabled: draft.enabled });
    }, [draft, form]);

    const submit = async () => {
        const value = await form.validateFields();
        await onSave({ ...draft, ...value });
        setDraft(null);
    };

    return (
        <>
            <Modal
                title="提示词来源"
                open={open}
                width={880}
                onCancel={() => !syncing && onClose()}
                mask={{ closable: !syncing }}
                footer={
                    <Space>
                        <Button loading={syncing} onClick={() => void onSyncAll()}>
                            全部同步
                        </Button>
                        <Button disabled={syncing} onClick={onClose}>
                            关闭
                        </Button>
                    </Space>
                }
            >
                <div className="mb-3 flex items-center justify-between gap-3">
                    <Typography.Text type="secondary">来源指向一个返回提示词数组的 JSON 地址，同步后提示词会进入对应分类，可在画布和提示词页直接使用。</Typography.Text>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => setDraft({ name: "", url: "", homepage: "", enabled: true })}>
                        新增来源
                    </Button>
                </div>
                <Table
                    rowKey="id"
                    size="small"
                    loading={loading}
                    dataSource={sources}
                    pagination={false}
                    scroll={{ y: 380 }}
                    columns={[
                        {
                            title: "名称",
                            dataIndex: "name",
                            width: 220,
                            render: (value: string, item) => (
                                <Space size={4}>
                                    <span>{value}</span>
                                    {item.builtIn ? <Tag className="m-0 text-[10px]">内置</Tag> : null}
                                    {!item.enabled ? <Tag className="m-0 text-[10px]">已停用</Tag> : null}
                                </Space>
                            ),
                        },
                        {
                            title: "地址",
                            dataIndex: "url",
                            render: (value: string, item) =>
                                item.kind === "builtin" ? (
                                    <Space size={4}>
                                        <Tag className="m-0 text-[10px]">内置数据</Tag>
                                        {item.homepage ? (
                                            <Typography.Link href={item.homepage} target="_blank" className="text-xs">
                                                数据来源
                                            </Typography.Link>
                                        ) : null}
                                    </Space>
                                ) : (
                                    <Space size={4}>
                                        <Typography.Text className="max-w-[320px] truncate" title={value}>
                                            {value}
                                        </Typography.Text>
                                        <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyText(value, "已复制地址")} />
                                        {item.homepage ? (
                                            <Typography.Link href={item.homepage} target="_blank" className="text-xs">
                                                主页
                                            </Typography.Link>
                                        ) : null}
                                    </Space>
                                ),
                        },
                        {
                            title: "状态",
                            key: "status",
                            width: 190,
                            render: (_, item) => (
                                <Space size={4} wrap>
                                    <Tag className="m-0 text-[10px]">{item.promptCount} 条</Tag>
                                    {item.lastError ? (
                                        <Tag color="error" className="m-0 text-[10px]" title={item.lastError}>
                                            同步失败
                                        </Tag>
                                    ) : item.lastSyncAt ? (
                                        <Tag color="success" className="m-0 text-[10px]">
                                            已同步
                                        </Tag>
                                    ) : (
                                        <Tag className="m-0 text-[10px]">未同步</Tag>
                                    )}
                                </Space>
                            ),
                        },
                        {
                            title: "操作",
                            key: "actions",
                            width: 210,
                            render: (_, item) => (
                                <Space size={4}>
                                    <Button size="small" icon={<ReloadOutlined />} loading={syncing} onClick={() => void onSync(item.id)}>
                                        同步
                                    </Button>
                                    <Button
                                        size="small"
                                        icon={<EditOutlined />}
                                        onClick={() => setDraft({ id: item.id, name: item.name, url: item.url, homepage: item.homepage, enabled: item.enabled })}
                                    >
                                        编辑
                                    </Button>
                                    <Button
                                        size="small"
                                        danger
                                        disabled={item.builtIn}
                                        icon={<DeleteOutlined />}
                                        onClick={() => Modal.confirm({ title: `删除「${item.name}」？`, content: "删除来源会同时移除它同步进来的提示词。", okText: "删除", okButtonProps: { danger: true }, cancelText: "取消", onOk: () => onDelete(item.id) })}
                                    />
                                </Space>
                            ),
                        },
                    ]}
                />
            </Modal>

            <Drawer
                title={draft?.id ? "编辑提示词来源" : "新增提示词来源"}
                open={Boolean(draft)}
                width={520}
                onClose={() => setDraft(null)}
                destroyOnHidden
                footer={
                    <Space>
                        <Button onClick={() => setDraft(null)}>取消</Button>
                        <Button type="primary" onClick={() => void submit()}>
                            保存
                        </Button>
                    </Space>
                }
            >
                <Form form={form} layout="vertical" requiredMark={false}>
                    <Form.Item name="name" label="来源名称" rules={[{ required: true, message: "请输入来源名称" }]}>
                        <Input placeholder="用于分类展示" />
                    </Form.Item>
                    <Form.Item
                        name="url"
                        label="JSON 地址"
                        rules={[
                            { required: true, message: "请输入 JSON 地址" },
                            {
                                validator: (_, value: string) =>
                                    !value || isHttpUrl(value.trim()) ? Promise.resolve() : Promise.reject(new Error("请输入有效的 JSON 地址")),
                            },
                        ]}
                    >
                        <Input placeholder="https://example.com/prompts.json" disabled={Boolean(draft?.id) && sources.some((item) => item.id === draft?.id && item.builtIn)} />
                    </Form.Item>
                    <Form.Item
                        name="homepage"
                        label="来源主页（可选）"
                        rules={[{ validator: (_, value: string) => !value || isHttpUrl(value.trim()) ? Promise.resolve() : Promise.reject(new Error("请输入有效的主页地址")) }]}
                    >
                        <Input placeholder="https://github.com/..." />
                    </Form.Item>
                    <Form.Item name="enabled" label="启用来源" valuePropName="checked" extra="停用后不会被「全部同步」拉取">
                        <Switch />
                    </Form.Item>
                    <Typography.Paragraph type="secondary" className="text-xs">
                        JSON 结构：数组，每项包含 title 与 prompt，可选 coverUrl、tags、preview、author、createdAt、updatedAt。字段名兼容 name/content/image 等常见别名。
                    </Typography.Paragraph>
                </Form>
            </Drawer>
        </>
    );
}
