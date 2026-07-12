import * as React from "react";
import {
  createRoleDefinition,
  deleteRoleDefinition,
  getRole,
  getRolePermissionOptions,
  listRoles,
  toggleRoleEnabled,
  updateRoleDefinition,
} from "../api/client";
import { Button } from "../components/ui/button";
import { Field, Form } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { zhCN } from "../i18n/zh-CN";
import type {
  PermissionOptions,
  RoleDefinitionPayload,
  RoleDetail,
  RoleListItem,
  RoleManagementSession,
  RolePermissionInput,
} from "./types";

const text = zhCN.roleAdmin;

type RoleManagementPageProps = {
  session: RoleManagementSession;
};

type EditorMode = "create" | "edit";
type FilterValue = "all" | "true" | "false";

const emptyPayload: RoleDefinitionPayload = {
  label: "",
  description: "",
  enabled: true,
  permissions: [],
  childRoleIds: [],
};

export function RoleManagementPage({ session }: RoleManagementPageProps) {
  const [roles, setRoles] = React.useState<RoleListItem[]>([]);
  const [canCreateFromAPI, setCanCreateFromAPI] = React.useState(false);
  const [search, setSearch] = React.useState("");
  const [systemFilter, setSystemFilter] = React.useState<FilterValue>("all");
  const [enabledFilter, setEnabledFilter] = React.useState<FilterValue>("all");
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [editorMode, setEditorMode] = React.useState<EditorMode>("create");
  const [editorRole, setEditorRole] = React.useState<RoleDetail | null>(null);
  const [editorOpen, setEditorOpen] = React.useState(false);
  const [permissionOptions, setPermissionOptions] = React.useState<PermissionOptions | null>(null);
  const [payload, setPayload] = React.useState<RoleDefinitionPayload>(emptyPayload);
  const [selectedPermissionKeys, setSelectedPermissionKeys] = React.useState<string[]>([]);
  const [conditionByKey, setConditionByKey] = React.useState<
    Record<string, RolePermissionInput["attributeConditions"]>
  >({});
  const [pendingToggle, setPendingToggle] = React.useState<RoleListItem | null>(null);

  const canCreateBySession = session.permissions.includes("Role.Create");
  const canCreate = canCreateBySession && canCreateFromAPI;

  const loadRoles = React.useCallback(async () => {
    try {
      const query: { search?: string; system?: boolean; enabled?: boolean } = {};
      if (search.trim()) {
        query.search = search.trim();
      }
      if (systemFilter !== "all") {
        query.system = systemFilter === "true";
      }
      if (enabledFilter !== "all") {
        query.enabled = enabledFilter === "true";
      }
      const { data, error } = await listRoles(query);
      if (error || !data) {
        setErrorMessage(text.errors.list);
        return;
      }
      setRoles((data.items ?? []) as RoleListItem[]);
      setCanCreateFromAPI(Boolean(data.canCreate));
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.list);
    }
  }, [enabledFilter, search, systemFilter]);

  React.useEffect(() => {
    void loadRoles();
  }, [loadRoles]);

  async function openCreateEditor() {
    setEditorMode("create");
    setEditorRole(null);
    setPayload(emptyPayload);
    setSelectedPermissionKeys([]);
    setConditionByKey({});
    setEditorOpen(true);
    setErrorMessage("");
    setSuccessMessage("");
    await loadPermissionOptions();
  }

  async function openEditEditor(role: RoleListItem) {
    setEditorMode("edit");
    setEditorOpen(true);
    setErrorMessage("");
    setSuccessMessage("");
    setPermissionOptions(null);
    try {
      const [optionsResult, roleResult] = await Promise.all([getRolePermissionOptions(), getRole(role.id)]);
      if (optionsResult.error || !optionsResult.data || roleResult.error || !roleResult.data) {
        setErrorMessage(text.errors.detail);
        return;
      }
      const detail = roleResult.data as RoleDetail;
      setPermissionOptions(optionsResult.data as PermissionOptions);
      setEditorRole(detail);
      setPayload({
        label: detail.label,
        description: detail.description ?? "",
        enabled: detail.enabled,
        permissions: detail.permissions ?? [],
        childRoleIds: detail.childRoleIds ?? [],
      });
      setSelectedPermissionKeys((detail.permissions ?? []).map((permission) => permissionKey(permission.resource, permission.action)));
      setConditionByKey(
        Object.fromEntries(
          (detail.permissions ?? []).map((permission) => [
            permissionKey(permission.resource, permission.action),
            permission.attributeConditions ?? {},
          ]),
        ),
      );
    } catch {
      setErrorMessage(text.errors.detail);
    }
  }

  async function loadPermissionOptions() {
    setPermissionOptions(null);
    try {
      const { data, error } = await getRolePermissionOptions();
      if (error || !data) {
        setErrorMessage(text.errors.options);
        return;
      }
      setPermissionOptions(data as PermissionOptions);
    } catch {
      setErrorMessage(text.errors.options);
    }
  }

  async function saveRole(event: React.FormEvent) {
    event.preventDefault();
    const body = {
      ...payload,
      label: payload.label.trim(),
      description: payload.description.trim(),
      permissions: buildPermissionPayload(permissionOptions, selectedPermissionKeys, conditionByKey),
    };
    try {
      const result =
        editorMode === "edit" && editorRole
          ? await updateRoleDefinition(editorRole.id, body)
          : await createRoleDefinition(body);
      if (result.error || !result.data) {
        throw new Error(readProblemCode(result.error) || "ROLE_SAVE_FAILED");
      }
      setEditorOpen(false);
      setSuccessMessage(editorMode === "edit" ? text.toasts.updated : text.toasts.created);
      setErrorMessage("");
      await loadRoles();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function removeRole(role: RoleListItem) {
    try {
      const { error } = await deleteRoleDefinition(role.id);
      if (error) {
        throw new Error(readProblemCode(error) || "ROLE_DELETE_FAILED");
      }
      setSuccessMessage(text.toasts.deleted);
      setErrorMessage("");
      await loadRoles();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function changeEnabled(role: RoleListItem, enabled: boolean) {
    if (role.isSystem && role.enabled && !enabled) {
      setPendingToggle(role);
      return;
    }
    await confirmToggle(role, enabled);
  }

  async function confirmToggle(role: RoleListItem, enabled: boolean) {
    try {
      const { error } = await toggleRoleEnabled(role.id, enabled);
      if (error) {
        throw new Error(readProblemCode(error) || "ROLE_TOGGLE_FAILED");
      }
      setPendingToggle(null);
      setSuccessMessage(enabled ? text.toasts.enabled : text.toasts.disabled);
      setErrorMessage("");
      await loadRoles();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-normal">{text.title}</h1>
          <p className="text-sm text-muted">{text.subtitle}</p>
        </div>
        <Button disabled={!canCreate} onClick={() => void openCreateEditor()} type="button" variant="primary">
          {text.actions.create}
        </Button>
      </div>

      {errorMessage ? (
        <p className="border border-red-400/30 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">
          {errorMessage}
        </p>
      ) : null}
      {successMessage ? (
        <p className="border border-accent/30 bg-accent/10 px-3 py-2 text-sm text-accent" role="status">
          {successMessage}
        </p>
      ) : null}

      <Form className="md:grid-cols-[1fr_180px_180px]" onSubmit={(event) => event.preventDefault()}>
        <Field label={text.filters.search}>
          <Input
            onChange={(event) => setSearch(event.target.value)}
            placeholder={text.filters.searchPlaceholder}
            value={search}
          />
        </Field>
        <Field label={text.filters.system}>
          <Select onChange={(event) => setSystemFilter(event.target.value as FilterValue)} value={systemFilter}>
            <option value="all">{text.filters.allSystems}</option>
            <option value="true">{text.system.system}</option>
            <option value="false">{text.system.custom}</option>
          </Select>
        </Field>
        <Field label={text.filters.enabled}>
          <Select onChange={(event) => setEnabledFilter(event.target.value as FilterValue)} value={enabledFilter}>
            <option value="all">{text.filters.allStatuses}</option>
            <option value="true">{text.status.enabled}</option>
            <option value="false">{text.status.disabled}</option>
          </Select>
        </Field>
      </Form>

      <div className="overflow-x-auto border border-white/10">
        <Table className="min-w-[960px]">
          <TableHeader>
            <TableRow>
              {[
                text.columns.role,
                text.columns.type,
                text.columns.status,
                text.columns.permissions,
                text.columns.children,
                text.columns.conditions,
                text.columns.references,
                text.columns.operations,
              ].map((label) => (
                <TableHead key={label}>
                  {label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {roles.length > 0 ? (
              roles.map((role) => (
                <TableRow key={role.id}>
                  <TableCell>
                    <div className="grid gap-1">
                      <span className="font-medium">{role.label}</span>
                      <span className="text-xs text-muted">{role.description || text.empty.value}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge>{role.isSystem ? text.system.system : text.system.custom}</Badge>
                  </TableCell>
                  <TableCell>{role.enabled ? text.status.enabled : text.status.disabled}</TableCell>
                  <TableCell>{role.permissionCount}</TableCell>
                  <TableCell>{role.childRoleCount}</TableCell>
                  <TableCell>{role.conditionSummary || text.empty.conditions}</TableCell>
                  <TableCell>
                    <div className="grid gap-1">
                      <span>{role.referenceCount}</span>
                      {!role.canDelete && role.referenceCount > 0 ? (
                        <span className="text-xs text-muted">{text.deleteBlocked(role.referenceCount)}</span>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-2">
                      {role.canEdit ? (
                        <Button onClick={() => void openEditEditor(role)} type="button">
                          {text.actions.edit}
                        </Button>
                      ) : null}
                      {role.canToggleEnabled ? (
                        <Button onClick={() => void changeEnabled(role, !role.enabled)} type="button">
                          {role.enabled ? text.actions.disable : text.actions.enable}
                        </Button>
                      ) : null}
                      <Button disabled={!role.canDelete} onClick={() => void removeRole(role)} type="button">
                        {text.actions.delete}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell className="py-8 text-center text-muted" colSpan={8}>
                  {text.empty.roles}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {pendingToggle ? (
        <aside className="grid gap-3 border border-amber-300/30 bg-amber-300/10 p-4" role="dialog">
          <h2 className="text-lg font-semibold tracking-normal">{text.confirmDisable.title}</h2>
          <p className="text-sm text-muted">{text.confirmDisable.description}</p>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void confirmToggle(pendingToggle, false)} type="button" variant="primary">
              {text.confirmDisable.confirm}
            </Button>
            <Button onClick={() => setPendingToggle(null)} type="button">
              {text.actions.cancel}
            </Button>
          </div>
        </aside>
      ) : null}

      {editorOpen ? (
        <RoleEditor
          availableRoles={roles.filter((role) => role.id !== editorRole?.id)}
          conditionByKey={conditionByKey}
          mode={editorMode}
          onCancel={() => setEditorOpen(false)}
          onConditionChange={setConditionByKey}
          onPermissionChange={setSelectedPermissionKeys}
          onPayloadChange={setPayload}
          onSubmit={saveRole}
          options={permissionOptions}
          payload={payload}
          role={editorRole}
          selectedPermissionKeys={selectedPermissionKeys}
        />
      ) : null}
    </div>
  );
}

function RoleEditor({
  availableRoles,
  conditionByKey,
  mode,
  onCancel,
  onConditionChange,
  onPayloadChange,
  onPermissionChange,
  onSubmit,
  options,
  payload,
  role,
  selectedPermissionKeys,
}: {
  availableRoles: RoleListItem[];
  conditionByKey: Record<string, RolePermissionInput["attributeConditions"]>;
  mode: EditorMode;
  onCancel: () => void;
  onConditionChange: React.Dispatch<React.SetStateAction<Record<string, RolePermissionInput["attributeConditions"]>>>;
  onPayloadChange: React.Dispatch<React.SetStateAction<RoleDefinitionPayload>>;
  onPermissionChange: React.Dispatch<React.SetStateAction<string[]>>;
  onSubmit: (event: React.FormEvent) => void;
  options: PermissionOptions | null;
  payload: RoleDefinitionPayload;
  role: RoleDetail | null;
  selectedPermissionKeys: string[];
}) {
  return (
    <aside aria-label={text.editor.label} className="grid gap-5 border border-white/10 bg-white/[0.03] p-4" role="dialog">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="grid gap-1">
          <h2 className="text-xl font-semibold tracking-normal">
            {mode === "edit" ? text.editor.editTitle : text.editor.createTitle}
          </h2>
          <p className="text-sm text-muted">{text.editor.subtitle}</p>
        </div>
        <Button onClick={onCancel} type="button">
          {text.actions.cancel}
        </Button>
      </div>

      <Form className="gap-5" onSubmit={onSubmit}>
        <div className="grid gap-4 md:grid-cols-[1fr_1fr_160px]">
          <Field label={text.editor.labelField}>
            <Input
              disabled={Boolean(role?.isSystem)}
              onChange={(event) => onPayloadChange((current) => ({ ...current, label: event.target.value }))}
              required
              value={payload.label}
            />
          </Field>
          <Field label={text.editor.descriptionField}>
            <Input
              onChange={(event) => onPayloadChange((current) => ({ ...current, description: event.target.value }))}
              value={payload.description}
            />
          </Field>
          <Field label={text.editor.enabledField}>
            <Select
              onChange={(event) => onPayloadChange((current) => ({ ...current, enabled: event.target.value === "true" }))}
              value={String(payload.enabled)}
            >
              <option value="true">{text.status.enabled}</option>
              <option value="false">{text.status.disabled}</option>
            </Select>
          </Field>
        </div>

        {options ? (
          <PermissionMatrix
            conditionByKey={conditionByKey}
            onConditionChange={onConditionChange}
            onPermissionChange={onPermissionChange}
            options={options}
            selectedPermissionKeys={selectedPermissionKeys}
          />
        ) : (
          <p className="text-sm text-muted">{text.editor.loadingOptions}</p>
        )}

        <section className="grid gap-3">
          <h3 className="text-sm font-semibold tracking-normal">{text.editor.childRoles}</h3>
          <div className="flex flex-wrap gap-3">
            {availableRoles.map((availableRole) => (
              <label className="inline-flex min-h-10 items-center gap-2 border border-white/10 bg-white/5 px-3 text-sm" key={availableRole.id}>
                <Input
                  checked={payload.childRoleIds.includes(availableRole.id)}
                  className="min-h-0 w-4"
                  onChange={(event) =>
                    onPayloadChange((current) => ({
                      ...current,
                      childRoleIds: toggleValue(current.childRoleIds, availableRole.id, event.target.checked),
                    }))
                  }
                  type="checkbox"
                />
                <span>
                  {text.editor.includeRole} {availableRole.label}
                </span>
              </label>
            ))}
            {availableRoles.length === 0 ? <p className="text-sm text-muted">{text.empty.childRoles}</p> : null}
          </div>
        </section>

        <div>
          <Button type="submit" variant="primary">
            {text.actions.save}
          </Button>
        </div>
      </Form>
    </aside>
  );
}

function PermissionMatrix({
  conditionByKey,
  onConditionChange,
  onPermissionChange,
  options,
  selectedPermissionKeys,
}: {
  conditionByKey: Record<string, RolePermissionInput["attributeConditions"]>;
  onConditionChange: React.Dispatch<React.SetStateAction<Record<string, RolePermissionInput["attributeConditions"]>>>;
  onPermissionChange: React.Dispatch<React.SetStateAction<string[]>>;
  options: PermissionOptions;
  selectedPermissionKeys: string[];
}) {
  const channelOptions = options.conditionOptions?.chan ?? ["social", "campus"];
  const expiredOptions = options.conditionOptions?.expired ?? [false, true];

  return (
    <section className="grid gap-3">
      <h3 className="text-sm font-semibold tracking-normal">{text.editor.permissions}</h3>
      <div className="grid gap-3">
        {options.resources.map((resource) => (
          <div className="grid gap-3 border border-white/10 bg-white/[0.02] p-3" key={resource.resource}>
            <h4 className="text-sm font-semibold tracking-normal">{resource.resource}</h4>
            <div className="grid gap-2 md:grid-cols-2">
              {resource.actions.map((action) => {
                const key = permissionKey(resource.resource, action.action);
                const checked = selectedPermissionKeys.includes(key);
                return (
                  <div className="grid gap-2 border border-white/10 bg-white/[0.03] p-3" key={key}>
                    <label className="inline-flex min-h-10 items-center gap-2 text-sm">
                      <Input
                        checked={checked}
                        className="min-h-0 w-4"
                        onChange={(event) =>
                          onPermissionChange((current) => toggleValue(current, key, event.target.checked))
                        }
                        type="checkbox"
                      />
                      <span>
                        {resource.resource} {action.action}
                      </span>
                    </label>
                    {checked && action.supportsConditions.chan ? (
                      <div className="flex flex-wrap gap-2">
                        {channelOptions.map((channel) => (
                          <label className="inline-flex items-center gap-2 text-xs text-muted" key={channel}>
                            <Input
                              checked={Boolean(conditionByKey[key]?.chan?.includes(channel))}
                              className="min-h-0 w-4"
                              onChange={(event) =>
                                onConditionChange((current) => ({
                                  ...current,
                                  [key]: {
                                    ...current[key],
                                    chan: toggleValue(current[key]?.chan ?? [], channel, event.target.checked),
                                  },
                                }))
                              }
                              type="checkbox"
                            />
                            <span>{text.conditions.chan[channel]}</span>
                          </label>
                        ))}
                      </div>
                    ) : null}
                    {checked && action.supportsConditions.expired ? (
                      <div className="flex flex-wrap gap-2">
                        {expiredOptions.map((expired) => (
                          <label className="inline-flex items-center gap-2 text-xs text-muted" key={String(expired)}>
                            <Input
                              checked={Boolean(conditionByKey[key]?.expired?.includes(expired))}
                              className="min-h-0 w-4"
                              onChange={(event) =>
                                onConditionChange((current) => ({
                                  ...current,
                                  [key]: {
                                    ...current[key],
                                    expired: toggleValue(current[key]?.expired ?? [], expired, event.target.checked),
                                  },
                                }))
                              }
                              type="checkbox"
                            />
                            <span>{text.conditions.expired[String(expired) as "false" | "true"]}</span>
                          </label>
                        ))}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function Badge({ children }: { children: React.ReactNode }) {
  return <span className="inline-flex border border-white/15 bg-white/5 px-2 py-1 text-xs">{children}</span>;
}

function buildPermissionPayload(
  options: PermissionOptions | null,
  selectedPermissionKeys: string[],
  conditionByKey: Record<string, RolePermissionInput["attributeConditions"]>,
) {
  if (!options) {
    return [];
  }
  return options.resources.flatMap((resource) =>
    resource.actions
      .filter((action) => selectedPermissionKeys.includes(permissionKey(resource.resource, action.action)))
      .map((action) => {
        const permission: RolePermissionInput = {
          resource: resource.resource,
          action: action.action,
        };
        const conditions = cleanConditions(conditionByKey[permissionKey(resource.resource, action.action)]);
        if (conditions) {
          permission.attributeConditions = conditions;
        }
        return permission;
      }),
  );
}

function cleanConditions(conditions: RolePermissionInput["attributeConditions"]) {
  if (!conditions) {
    return undefined;
  }
  const cleaned: RolePermissionInput["attributeConditions"] = {};
  if (conditions.chan && conditions.chan.length > 0) {
    cleaned.chan = conditions.chan;
  }
  if (conditions.expired && conditions.expired.length > 0) {
    cleaned.expired = conditions.expired;
  }
  if (typeof conditions.self === "boolean") {
    cleaned.self = conditions.self;
  }
  return Object.keys(cleaned).length > 0 ? cleaned : undefined;
}

function permissionKey(resource: string, action: string) {
  return `${resource}:${action}`;
}

function toggleValue<T>(values: T[], value: T, checked: boolean) {
  if (checked) {
    return values.includes(value) ? values : [...values, value];
  }
  return values.filter((item) => item !== value);
}

function readProblemCode(error: unknown) {
  if (!error || typeof error !== "object" || !("code" in error)) {
    return "";
  }
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : "";
}

function formatWorkflowError(error: unknown) {
  const code = error instanceof Error ? error.message : "";
  return text.errors.codes[code as keyof typeof text.errors.codes] ?? text.errors.generic;
}
