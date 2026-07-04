import * as React from "react";
import {
  createDepartment,
  createPosition,
  deleteDepartment,
  deletePosition,
  getDepartment,
  getPosition,
  listDepartments,
  listPositions,
  updateDepartment,
  updatePosition,
  type PositionMutation,
} from "../api/client";
import { Button } from "../components/ui/button";
import { Field, Form } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { zhCN } from "../i18n/zh-CN";
import type { DepartmentDetail, DepartmentListItem, DepartmentPositionSession, PositionDetail, PositionListItem } from "./types";

const text = zhCN.departmentPosition;

const channelLabels: Record<string, string> = {
  campus: zhCN.session.workspace.channels.campus,
  social: zhCN.session.workspace.channels.social,
};

const statusLabels: Record<string, string> = {
  off: text.status.off,
  on: text.status.on,
};

type DepartmentPositionPageProps = {
  session: DepartmentPositionSession;
};

export function DepartmentPositionPage({ session }: DepartmentPositionPageProps) {
  const [activeTab, setActiveTab] = React.useState<"departments" | "positions">("departments");
  const [departments, setDepartments] = React.useState<DepartmentListItem[]>([]);
  const [positions, setPositions] = React.useState<PositionListItem[]>([]);
  const [departmentDetail, setDepartmentDetail] = React.useState<DepartmentDetail | null>(null);
  const [positionDetail, setPositionDetail] = React.useState<PositionDetail | null>(null);
  const [departmentFilter, setDepartmentFilter] = React.useState("");
  const [channelFilter, setChannelFilter] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [search, setSearch] = React.useState("");
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [departmentForm, setDepartmentForm] = React.useState<DepartmentFormState | null>(null);
  const [positionForm, setPositionForm] = React.useState<PositionFormState | null>(null);

  const loadDepartments = React.useCallback(async (isCurrent: () => boolean = () => true) => {
    try {
      const { data, error } = await listDepartments({});
      if (!isCurrent()) {
        return;
      }
      if (error || !data) {
        setErrorMessage(text.errors.departments);
        return;
      }
      setDepartments((data.items ?? []) as DepartmentListItem[]);
      setErrorMessage("");
    } catch {
      if (isCurrent()) {
        setErrorMessage(text.errors.departments);
      }
    }
  }, []);

  const loadPositions = React.useCallback(
    async (isCurrent: () => boolean = () => true) => {
      try {
        const { data, error } = await listPositions({
          departmentId: departmentFilter || undefined,
          chan: channelFilter === "social" || channelFilter === "campus" ? channelFilter : undefined,
          status: statusFilter === "on" || statusFilter === "off" ? statusFilter : undefined,
          search: search.trim() || undefined,
        });
        if (!isCurrent()) {
          return;
        }
        if (error || !data) {
          setErrorMessage(text.errors.positions);
          return;
        }
        setPositions((data.items ?? []) as PositionListItem[]);
        setErrorMessage("");
      } catch {
        if (isCurrent()) {
          setErrorMessage(text.errors.positions);
        }
      }
    },
    [channelFilter, departmentFilter, search, statusFilter],
  );

  React.useEffect(() => {
    let isCurrent = true;
    void loadDepartments(() => isCurrent);
    return () => {
      isCurrent = false;
    };
  }, [loadDepartments]);

  React.useEffect(() => {
    let isCurrent = true;
    void loadPositions(() => isCurrent);
    return () => {
      isCurrent = false;
    };
  }, [loadPositions]);

  async function handleOpenDepartment(id: string) {
    try {
      const { data, error } = await getDepartment(id);
      if (error || !data) {
        setErrorMessage(text.errors.departmentDetail);
        setSuccessMessage("");
        return;
      }
      setDepartmentDetail(data as DepartmentDetail);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.departmentDetail);
      setSuccessMessage("");
    }
  }

  async function handleOpenPosition(id: string) {
    try {
      const { data, error } = await getPosition(id);
      if (error || !data) {
        setErrorMessage(text.errors.positionDetail);
        setSuccessMessage("");
        return;
      }
      setPositionDetail(data as PositionDetail);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.positionDetail);
      setSuccessMessage("");
    }
  }

  async function handleSubmitDepartment(event: React.FormEvent) {
    event.preventDefault();
    if (!departmentForm) {
      return;
    }
    const name = departmentForm.name.trim();
    if (!name) {
      setErrorMessage(text.errors.codes.DEPARTMENT_NAME_REQUIRED);
      setSuccessMessage("");
      return;
    }
    try {
      const result =
        departmentForm.mode === "create"
          ? await createDepartment({ name })
          : await updateDepartment(departmentForm.id, { name });
      if (result.error) {
        throw new Error(readProblemCode(result.error) || "DEPARTMENT_SAVE_FAILED");
      }
      setDepartmentForm(null);
      setErrorMessage("");
      setSuccessMessage(departmentForm.mode === "create" ? text.toasts.departmentCreated : text.toasts.departmentUpdated);
      await loadDepartments();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleDeleteDepartment(id: string) {
    try {
      const { error } = await deleteDepartment(id);
      if (error) {
        throw new Error(readProblemCode(error) || "DEPARTMENT_DELETE_FAILED");
      }
      setErrorMessage("");
      setSuccessMessage(text.toasts.departmentDeleted);
      await loadDepartments();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleSubmitPosition(event: React.FormEvent) {
    event.preventDefault();
    if (!positionForm) {
      return;
    }
    const parsed = parsePositionForm(positionForm.values);
    if ("error" in parsed) {
      setErrorMessage(parsed.error);
      setSuccessMessage("");
      return;
    }
    try {
      if (positionForm.mode === "update" && !positionForm.id) {
        throw new Error("POSITION_SAVE_FAILED");
      }
      const result =
        positionForm.mode === "create"
          ? await createPosition(parsed.value)
          : await updatePosition(positionForm.id, parsed.value);
      if (result.error) {
        throw new Error(readProblemCode(result.error) || "POSITION_SAVE_FAILED");
      }
      setPositionForm(null);
      setErrorMessage("");
      setSuccessMessage(positionForm.mode === "create" ? text.toasts.positionCreated : text.toasts.positionUpdated);
      await loadPositions();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function openEditPosition(position: PositionListItem) {
    const detail = await loadPositionDetail(position.id);
    if (!detail) {
      return;
    }
    setPositionForm({ mode: "update", id: position.id, values: positionDetailToForm(detail) });
  }

  async function handleTogglePosition(position: PositionListItem) {
    const detail = await loadPositionDetail(position.id);
    if (!detail) {
      return;
    }
    const nextStatus = detail.status === "on" ? "off" : "on";
    const body = { ...positionDetailToMutation(detail), status: nextStatus as "on" | "off" };
    try {
      const { error } = await updatePosition(position.id, body);
      if (error) {
        throw new Error(readProblemCode(error) || "POSITION_SAVE_FAILED");
      }
      setErrorMessage("");
      setSuccessMessage(nextStatus === "on" ? text.toasts.positionOn : text.toasts.positionOff);
      await loadPositions();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleDeletePosition(id: string) {
    try {
      const { error } = await deletePosition(id);
      if (error) {
        throw new Error(readProblemCode(error) || "POSITION_DELETE_FAILED");
      }
      setErrorMessage("");
      setSuccessMessage(text.toasts.positionDeleted);
      await loadPositions();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function loadPositionDetail(id: string) {
    try {
      const { data, error } = await getPosition(id);
      if (error || !data) {
        setErrorMessage(text.errors.positionDetail);
        setSuccessMessage("");
        return null;
      }
      return data as PositionDetail;
    } catch {
      setErrorMessage(text.errors.positionDetail);
      setSuccessMessage("");
      return null;
    }
  }

  const visibleDepartments = departments;
  const visiblePositions = positions;
  const canCreateDepartment = session.permissions.includes("Department.Create");
  const canUpdateDepartment = session.permissions.includes("Department.Update");
  const canDeleteDepartment = session.permissions.includes("Department.Delete");
  const canCreatePosition = session.permissions.includes("Position.Create");
  const canUpdatePosition = session.permissions.includes("Position.Update");
  const canDeletePosition = session.permissions.includes("Position.Delete");

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-normal">{text.title}</h1>
          <p className="text-sm text-muted">{text.subtitle}</p>
        </div>
        <div className="border border-accent/40 bg-accent/10 px-3 py-2 text-sm">
          <span className="text-muted">{text.dataScopeLabel}</span>
          <span>{formatDepartmentScope(session)}</span>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button
          aria-pressed={activeTab === "departments"}
          onClick={() => setActiveTab("departments")}
          type="button"
          variant={activeTab === "departments" ? "primary" : "secondary"}
        >
          {text.tabs.departments}
        </Button>
        <Button
          aria-pressed={activeTab === "positions"}
          onClick={() => setActiveTab("positions")}
          type="button"
          variant={activeTab === "positions" ? "primary" : "secondary"}
        >
          {text.tabs.positions}
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

      {activeTab === "departments" ? (
        <DepartmentTab
          canCreate={canCreateDepartment}
          canDelete={canDeleteDepartment}
          canUpdate={canUpdateDepartment}
          departments={visibleDepartments}
          detail={departmentDetail}
          form={departmentForm}
          onCancelForm={() => setDepartmentForm(null)}
          onDelete={(id) => void handleDeleteDepartment(id)}
          onEdit={(department) => setDepartmentForm({ mode: "update", id: department.id, name: department.name })}
          onOpen={(id) => void handleOpenDepartment(id)}
          onSetForm={setDepartmentForm}
          onSubmitForm={(event) => void handleSubmitDepartment(event)}
        />
      ) : (
        <PositionTab
          canCreate={canCreatePosition}
          canDelete={canDeletePosition}
          canUpdate={canUpdatePosition}
          departmentFilter={departmentFilter}
          departments={departments}
          detail={positionDetail}
          form={positionForm}
          onChannelChange={setChannelFilter}
          onCancelForm={() => setPositionForm(null)}
          onCreate={() => setPositionForm({ mode: "create", values: emptyPositionForm(defaultDepartmentID(departments)) })}
          onDelete={(id) => void handleDeletePosition(id)}
          onDepartmentChange={setDepartmentFilter}
          onEdit={(position) => void openEditPosition(position)}
          onOpen={(id) => void handleOpenPosition(id)}
          onSearchChange={setSearch}
          onSetForm={setPositionForm}
          onStatusChange={setStatusFilter}
          onSubmitForm={(event) => void handleSubmitPosition(event)}
          onToggle={(position) => void handleTogglePosition(position)}
          positions={visiblePositions}
          search={search}
          channelFilter={channelFilter}
          statusFilter={statusFilter}
        />
      )}
    </div>
  );
}

function DepartmentTab({
  canCreate,
  canDelete,
  canUpdate,
  departments,
  detail,
  form,
  onCancelForm,
  onDelete,
  onEdit,
  onOpen,
  onSetForm,
  onSubmitForm,
}: {
  canCreate: boolean;
  canDelete: boolean;
  canUpdate: boolean;
  departments: DepartmentListItem[];
  detail: DepartmentDetail | null;
  form: DepartmentFormState | null;
  onCancelForm: () => void;
  onDelete: (id: string) => void;
  onEdit: (department: DepartmentListItem) => void;
  onOpen: (id: string) => void;
  onSetForm: (form: DepartmentFormState) => void;
  onSubmitForm: (event: React.FormEvent) => void;
}) {
  return (
    <div className="grid gap-4">
      {canCreate ? (
        <div>
          <Button onClick={() => onSetForm({ mode: "create", id: "", name: "" })} type="button" variant="primary">
            {text.actions.createDepartment}
          </Button>
        </div>
      ) : null}
      {form ? (
        <Form className="border border-white/10 p-3 md:grid-cols-[1fr_auto_auto]" onSubmit={onSubmitForm}>
          <Field label={text.department.form.name}>
            <Input onChange={(event) => onSetForm({ ...form, name: event.target.value })} value={form.name} />
          </Field>
          <Button type="submit" variant="primary">
            {text.actions.save}
          </Button>
          <Button onClick={onCancelForm} type="button">
            {text.actions.cancel}
          </Button>
        </Form>
      ) : null}
      <div className="overflow-x-auto border border-white/10">
        <table className="w-full min-w-[760px] border-collapse text-left text-sm">
          <thead className="bg-white/[0.04] text-muted">
            <tr>
              {[text.department.columns.name, text.department.columns.positionCount, text.department.columns.resumeCount, text.common.updatedAt, text.common.operations].map((label) => (
                <th className="border-b border-white/10 px-3 py-3 font-medium" key={label} scope="col">
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {departments.length > 0 ? (
              departments.map((department) => (
                <tr className="border-b border-white/10 last:border-0" key={department.id}>
                  <td className="px-3 py-3 font-medium">{department.name}</td>
                  <td className="px-3 py-3">{department.positionCount}</td>
                  <td className="px-3 py-3">{department.resumeCount}</td>
                  <td className="px-3 py-3">{formatDate(department.updatedAt)}</td>
                  <td className="px-3 py-3">
                    <div className="flex flex-wrap gap-2">
                      {department.canGet ? (
                        <Button onClick={() => onOpen(department.id)} type="button">
                          {text.actions.view}
                        </Button>
                      ) : null}
                      {canUpdate && department.canUpdate ? (
                        <Button onClick={() => onEdit(department)} type="button">
                          {text.actions.edit}
                        </Button>
                      ) : null}
                      {canDelete && department.canDelete ? (
                        <Button onClick={() => onDelete(department.id)} type="button">
                          {text.actions.delete}
                        </Button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-3 py-8 text-center text-muted" colSpan={5}>
                  {text.empty.departments}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {detail ? <DepartmentDetailPanel detail={detail} /> : null}
    </div>
  );
}

function DepartmentDetailPanel({ detail }: { detail: DepartmentDetail }) {
  return (
    <aside aria-label={text.department.detailLabel} className="grid gap-4 border border-white/10 bg-white/[0.03] p-4">
      <div className="grid gap-2">
        <h2 className="text-xl font-semibold tracking-normal">{detail.name}</h2>
        <dl className="grid gap-2 text-sm sm:grid-cols-3">
          <DetailField label={text.department.columns.positionCount} value={String(detail.positionCount)} />
          <DetailField label={text.department.columns.resumeCount} value={String(detail.resumeCount)} />
          <DetailField label={text.common.updatedAt} value={formatDate(detail.updatedAt)} />
        </dl>
      </div>
      <section className="grid gap-2">
        <h3 className="text-sm font-semibold tracking-normal">{text.department.positions}</h3>
        {detail.positions.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {detail.positions.map((position) => (
              <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={position.id}>
                {position.name}
              </span>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted">{text.empty.positions}</p>
        )}
      </section>
    </aside>
  );
}

function PositionTab({
  canCreate,
  canDelete,
  canUpdate,
  channelFilter,
  departmentFilter,
  departments,
  detail,
  form,
  onChannelChange,
  onCancelForm,
  onCreate,
  onDelete,
  onDepartmentChange,
  onEdit,
  onOpen,
  onSearchChange,
  onSetForm,
  onStatusChange,
  onSubmitForm,
  onToggle,
  positions,
  search,
  statusFilter,
}: {
  canCreate: boolean;
  canDelete: boolean;
  canUpdate: boolean;
  channelFilter: string;
  departmentFilter: string;
  departments: DepartmentListItem[];
  detail: PositionDetail | null;
  form: PositionFormState | null;
  onChannelChange: (value: string) => void;
  onCancelForm: () => void;
  onCreate: () => void;
  onDelete: (id: string) => void;
  onDepartmentChange: (value: string) => void;
  onEdit: (position: PositionListItem) => void;
  onOpen: (id: string) => void;
  onSearchChange: (value: string) => void;
  onSetForm: (form: PositionFormState) => void;
  onStatusChange: (value: string) => void;
  onSubmitForm: (event: React.FormEvent) => void;
  onToggle: (position: PositionListItem) => void;
  positions: PositionListItem[];
  search: string;
  statusFilter: string;
}) {
  return (
    <div className="grid gap-4">
      {canCreate ? (
        <div>
          <Button onClick={onCreate} type="button" variant="primary">
            {text.actions.createPosition}
          </Button>
        </div>
      ) : null}
      {form ? (
        <PositionForm
          departments={departments}
          form={form}
          onCancel={onCancelForm}
          onChange={onSetForm}
          onSubmit={onSubmitForm}
        />
      ) : (
        <div className="grid gap-3 md:grid-cols-4">
          <Field label={text.filters.department}>
            <Select onChange={(event) => onDepartmentChange(event.target.value)} value={departmentFilter}>
              <option value="">{text.filters.allDepartments}</option>
              {departments.map((department) => (
                <option key={department.id} value={department.id}>
                  {department.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={text.filters.channel}>
            <Select onChange={(event) => onChannelChange(event.target.value)} value={channelFilter}>
              <option value="">{text.filters.allChannels}</option>
              <option value="social">{channelLabels.social}</option>
              <option value="campus">{channelLabels.campus}</option>
            </Select>
          </Field>
          <Field label={text.filters.status}>
            <Select onChange={(event) => onStatusChange(event.target.value)} value={statusFilter}>
              <option value="">{text.filters.allStatuses}</option>
              <option value="on">{text.status.on}</option>
              <option value="off">{text.status.off}</option>
            </Select>
          </Field>
          <Field label={text.filters.search}>
            <Input onChange={(event) => onSearchChange(event.target.value)} placeholder={text.filters.searchPlaceholder} type="search" value={search} />
          </Field>
        </div>
      )}

      <div className="overflow-x-auto border border-white/10">
        <table className="w-full min-w-[980px] border-collapse text-left text-sm">
          <thead className="bg-white/[0.04] text-muted">
            <tr>
              {[text.position.columns.name, text.position.columns.department, text.position.columns.channel, text.position.columns.level, text.position.columns.status, text.position.columns.keywordCount, text.position.columns.implicitTagCount, text.common.operations].map((label) => (
                <th className="border-b border-white/10 px-3 py-3 font-medium" key={label} scope="col">
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {positions.length > 0 ? (
              positions.map((position) => (
                <tr className="border-b border-white/10 last:border-0" key={position.id}>
                  <td className="px-3 py-3 font-medium">{position.name}</td>
                  <td className="px-3 py-3">{position.department.name}</td>
                  <td className="px-3 py-3">{channelLabels[position.chan] ?? position.chan}</td>
                  <td className="px-3 py-3">{position.level || text.empty.value}</td>
                  <td className="px-3 py-3">{statusLabels[position.status] ?? position.status}</td>
                  <td className="px-3 py-3">{position.keywordCount}</td>
                  <td className="px-3 py-3">{position.implicitTagCount}</td>
                  <td className="px-3 py-3">
                    <div className="flex flex-wrap gap-2">
                      {position.canGet ? (
                        <Button onClick={() => onOpen(position.id)} type="button">
                          {text.actions.view}
                        </Button>
                      ) : null}
                      {canUpdate && position.canUpdate ? (
                        <Button onClick={() => onEdit(position)} type="button">
                          {text.actions.edit}
                        </Button>
                      ) : null}
                      {canUpdate && position.canUpdate ? (
                        <Button onClick={() => onToggle(position)} type="button">
                          {position.status === "on" ? text.actions.off : text.actions.on}
                        </Button>
                      ) : null}
                      {canDelete && position.canDelete ? (
                        <Button onClick={() => onDelete(position.id)} type="button">
                          {text.actions.delete}
                        </Button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-3 py-8 text-center text-muted" colSpan={8}>
                  {search.trim() ? text.empty.search : text.empty.positions}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {detail ? <PositionDetailPanel detail={detail} /> : null}
    </div>
  );
}

function PositionForm({
  departments,
  form,
  onCancel,
  onChange,
  onSubmit,
}: {
  departments: DepartmentListItem[];
  form: PositionFormState;
  onCancel: () => void;
  onChange: (form: PositionFormState) => void;
  onSubmit: (event: React.FormEvent) => void;
}) {
  const values = form.values;
  const updateValues = (patch: Partial<PositionFormValues>) => onChange({ ...form, values: { ...values, ...patch } });

  return (
    <Form className="border border-white/10 p-3 md:grid-cols-2" onSubmit={onSubmit}>
      <Field label={text.position.form.name}>
        <Input onChange={(event) => updateValues({ name: event.target.value })} value={values.name} />
      </Field>
      <Field label={text.position.form.department}>
        <Select onChange={(event) => updateValues({ departmentId: event.target.value })} value={values.departmentId}>
          <option value="">{text.filters.allDepartments}</option>
          {departments.map((department) => (
            <option key={department.id} value={department.id}>
              {department.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label={text.position.form.channel}>
        <Select onChange={(event) => updateValues({ chan: event.target.value })} value={values.chan}>
          <option value="social">{channelLabels.social}</option>
          <option value="campus">{channelLabels.campus}</option>
        </Select>
      </Field>
      <Field label={text.position.form.level}>
        <Input onChange={(event) => updateValues({ level: event.target.value })} value={values.level} />
      </Field>
      <Field label={text.position.form.status}>
        <Select onChange={(event) => updateValues({ status: event.target.value })} value={values.status}>
          <option value="on">{text.status.on}</option>
          <option value="off">{text.status.off}</option>
        </Select>
      </Field>
      <Field label={text.position.form.duties}>
        <Input onChange={(event) => updateValues({ duties: event.target.value })} value={values.duties} />
      </Field>
      <Field label={text.position.form.must}>
        <Input onChange={(event) => updateValues({ must: event.target.value })} value={values.must} />
      </Field>
      <Field label={text.position.form.keywords}>
        <Input onChange={(event) => updateValues({ keywords: event.target.value })} value={values.keywords} />
      </Field>
      <Field label={text.position.form.implicitTags}>
        <Input onChange={(event) => updateValues({ implicitTags: event.target.value })} value={values.implicitTags} />
      </Field>
      <div className="flex flex-wrap gap-2 md:col-span-2">
        <Button type="submit" variant="primary">
          {text.actions.save}
        </Button>
        <Button onClick={onCancel} type="button">
          {text.actions.cancel}
        </Button>
      </div>
    </Form>
  );
}

function PositionDetailPanel({ detail }: { detail: PositionDetail }) {
  return (
    <aside aria-label={text.position.detailLabel} className="grid gap-4 border border-white/10 bg-white/[0.03] p-4">
      <div className="grid gap-2">
        <h2 className="text-xl font-semibold tracking-normal">{detail.name}</h2>
        <dl className="grid gap-2 text-sm sm:grid-cols-4">
          <DetailField label={text.position.columns.department} value={detail.department.name} />
          <DetailField label={text.position.columns.channel} value={channelLabels[detail.chan] ?? detail.chan} />
          <DetailField label={text.position.columns.level} value={detail.level || text.empty.value} />
          <DetailField label={text.position.columns.status} value={statusLabels[detail.status] ?? detail.status} />
        </dl>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <ListSection items={detail.duties} title={text.position.sections.duties} />
        <ListSection items={detail.must} title={text.position.sections.must} />
        <ChipSection items={detail.keywords} title={text.position.sections.keywords} />
        <ChipSection items={detail.implicitTags.map((tag) => `${tag.name} · ${tag.w}`)} title={text.position.sections.implicitTags} />
      </div>
    </aside>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1">
      <dt className="text-muted">{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function ListSection({ items, title }: { items: string[]; title: string }) {
  return (
    <section className="grid gap-2 border border-white/10 p-3">
      <h3 className="text-sm font-semibold tracking-normal">{title}</h3>
      {items.length > 0 ? (
        <ul className="grid gap-1 text-sm text-muted">
          {items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted">{text.empty.value}</p>
      )}
    </section>
  );
}

function ChipSection({ items, title }: { items: string[]; title: string }) {
  return (
    <section className="grid gap-2 border border-white/10 p-3">
      <h3 className="text-sm font-semibold tracking-normal">{title}</h3>
      {items.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {items.map((item) => (
            <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={item}>
              {item}
            </span>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted">{text.empty.value}</p>
      )}
    </section>
  );
}

function formatDepartmentScope(session: DepartmentPositionSession) {
  if (session.dataScope.allDepartments) {
    return zhCN.session.workspace.allDepartments;
  }
  if (session.dataScope.departments.length === 0) {
    return zhCN.session.workspace.noDepartmentScope;
  }
  return session.dataScope.departments.map((department) => department.name).join("、");
}

function formatDate(value: string) {
  if (!value) {
    return text.empty.value;
  }
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

type DepartmentFormState = {
  mode: "create" | "update";
  id: string;
  name: string;
};

type PositionFormState =
  | { mode: "create"; values: PositionFormValues }
  | { mode: "update"; id: string; values: PositionFormValues };

type PositionFormValues = {
  name: string;
  departmentId: string;
  chan: string;
  level: string;
  status: string;
  duties: string;
  must: string;
  keywords: string;
  implicitTags: string;
};

function emptyPositionForm(departmentId: string): PositionFormValues {
  return {
    name: "",
    departmentId,
    chan: "social",
    level: "",
    status: "on",
    duties: "",
    must: "",
    keywords: "",
    implicitTags: "",
  };
}

function defaultDepartmentID(departments: DepartmentListItem[]) {
  return departments.length === 1 ? departments[0].id : "";
}

function positionDetailToForm(detail: PositionDetail): PositionFormValues {
  return {
    name: detail.name,
    departmentId: detail.department.id,
    chan: detail.chan,
    level: detail.level,
    status: detail.status,
    duties: detail.duties.join(","),
    must: detail.must.join(","),
    keywords: detail.keywords.join(","),
    implicitTags: detail.implicitTags.map((tag) => `${tag.name}:${tag.w}`).join(","),
  };
}

function positionDetailToMutation(detail: PositionDetail): PositionMutation {
  return {
    name: detail.name,
    departmentId: detail.department.id,
    chan: detail.chan === "campus" ? "campus" : "social",
    level: detail.level,
    status: detail.status === "off" ? "off" : "on",
    duties: detail.duties,
    must: detail.must,
    keywords: detail.keywords,
    implicitTags: detail.implicitTags,
  };
}

function parsePositionForm(values: PositionFormValues): { value: PositionMutation } | { error: string } {
  const keywords = splitList(values.keywords);
  const keywordSet = new Set<string>();
  for (const keyword of keywords) {
    const key = keyword.toLocaleLowerCase();
    if (keywordSet.has(key)) {
      return { error: text.errors.codes.POSITION_DUPLICATE_KEYWORD };
    }
    keywordSet.add(key);
  }

  const implicitTags = parseImplicitTags(values.implicitTags);
  if ("error" in implicitTags) {
    return implicitTags;
  }
  const tagSet = new Set<string>();
  for (const tag of implicitTags.value) {
    const key = tag.name.toLocaleLowerCase();
    if (tagSet.has(key)) {
      return { error: text.errors.codes.POSITION_DUPLICATE_IMPLICIT_TAG };
    }
    tagSet.add(key);
  }

  return {
    value: {
      name: values.name.trim(),
      departmentId: values.departmentId,
      chan: values.chan === "campus" ? "campus" : "social",
      level: values.level.trim(),
      status: values.status === "off" ? "off" : "on",
      duties: splitList(values.duties),
      must: splitList(values.must),
      keywords,
      implicitTags: implicitTags.value,
    },
  };
}

function splitList(value: string) {
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseImplicitTags(value: string): { value: Array<{ name: string; w: number }> } | { error: string } {
  const tags = splitList(value).map((item) => {
    const [name, rawWeight] = item.split(":");
    const weight = rawWeight ? Number(rawWeight.trim()) : 40;
    return { name: name.trim(), w: weight };
  });
  if (tags.some((tag) => Number.isNaN(tag.w) || tag.w < 0 || tag.w > 100)) {
    return { error: text.errors.codes.POSITION_INVALID_IMPLICIT_WEIGHT };
  }
  return { value: tags.filter((tag) => tag.name) };
}

function readProblemCode(error: unknown) {
  if (error && typeof error === "object" && "code" in error && typeof error.code === "string") {
    return error.code;
  }
  return "";
}

function formatWorkflowError(error: unknown) {
  const code = error instanceof Error ? error.message : "";
  return text.errors.codes[code as keyof typeof text.errors.codes] ?? text.errors.generic;
}
