import * as React from "react";
import {
  assignUserRoleBindings,
  deleteUserRoleBinding,
  listAssignableRoles,
  listDepartments,
  listUsers,
} from "../api/client";
import { Button } from "../components/ui/button";
import { Field, Form } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
import { zhCN } from "../i18n/zh-CN";
import type { AssignableRole, DepartmentOption, ManagedUser, PendingBinding, UserRoleBinding, UsersSession } from "./types";

const text = zhCN.userAdmin;

type UsersPageProps = {
  session: UsersSession;
};

export function UsersPage({ session }: UsersPageProps) {
  const [users, setUsers] = React.useState<ManagedUser[]>([]);
  const [search, setSearch] = React.useState("");
  const [dataScopeSummary, setDataScopeSummary] = React.useState("");
  const [canAssignFromAPI, setCanAssignFromAPI] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [activeUser, setActiveUser] = React.useState<ManagedUser | null>(null);
  const [roles, setRoles] = React.useState<AssignableRole[]>([]);
  const [departments, setDepartments] = React.useState<DepartmentOption[]>([]);
  const [selectedRoleID, setSelectedRoleID] = React.useState("");
  const [selectedDepartmentID, setSelectedDepartmentID] = React.useState("");
  const [pendingBindings, setPendingBindings] = React.useState<PendingBinding[]>([]);

  const canAssignBySession = session.permissions.includes("UserDepartmentRole.Create");
  const canDeleteBySession = session.permissions.includes("UserDepartmentRole.Delete");
  const canAssign = canAssignBySession && canAssignFromAPI;

  const loadUsers = React.useCallback(async () => {
    try {
      const query = search.trim() ? { search: search.trim() } : {};
      const { data, error } = await listUsers(query);
      if (error || !data) {
        setErrorMessage(text.errors.list);
        return;
      }
      setUsers((data.items ?? []) as ManagedUser[]);
      setDataScopeSummary(data.dataScopeSummary ?? "");
      setCanAssignFromAPI(Boolean(data.canAssignRoles));
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.list);
    }
  }, [search]);

  React.useEffect(() => {
    let isCurrent = true;
    async function run() {
      if (!isCurrent) {
        return;
      }
      await loadUsers();
    }
    void run();
    return () => {
      isCurrent = false;
    };
  }, [loadUsers]);

  async function openAssignment(user: ManagedUser) {
    setActiveUser(user);
    setPendingBindings([]);
    setErrorMessage("");
    setSuccessMessage("");
    try {
      const [roleResult, departmentResult] = await Promise.all([listAssignableRoles(), listDepartments({})]);
      if (roleResult.error || !roleResult.data || departmentResult.error || !departmentResult.data) {
        setErrorMessage(text.errors.options);
        return;
      }
      const nextRoles = (roleResult.data.items ?? []) as AssignableRole[];
      const nextDepartments = (departmentResult.data.items ?? []) as DepartmentOption[];
      setRoles(nextRoles);
      setDepartments(nextDepartments);
      setSelectedRoleID(nextRoles[0]?.id ?? "");
      setSelectedDepartmentID(nextDepartments[0]?.id ?? "");
    } catch {
      setErrorMessage(text.errors.options);
    }
  }

  function addPendingBinding() {
    const roleId = selectedRoleID || roles[0]?.id || "";
    const departmentId = selectedDepartmentID || departments[0]?.id || "";
    if (!roleId || !departmentId) {
      setErrorMessage(text.errors.options);
      return;
    }
    if (pendingBindings.some((binding) => binding.roleId === roleId && binding.departmentId === departmentId)) {
      setErrorMessage(text.errors.duplicatePending);
      setSuccessMessage("");
      return;
    }
    setPendingBindings([...pendingBindings, { roleId, departmentId }]);
    setErrorMessage("");
  }

  async function savePendingBindings() {
    if (!activeUser || pendingBindings.length === 0) {
      setErrorMessage(text.errors.emptyPending);
      setSuccessMessage("");
      return;
    }
    try {
      const { data, error } = await assignUserRoleBindings(activeUser.id, { bindings: pendingBindings });
      if (error || !data) {
        throw new Error(readProblemCode(error) || "USER_ROLE_BINDING_SAVE_FAILED");
      }
      setActiveUser(null);
      setPendingBindings([]);
      setSuccessMessage(data.message || text.toasts.assigned);
      setErrorMessage("");
      await loadUsers();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function removeBinding(user: ManagedUser, binding: UserRoleBinding) {
    try {
      const { data, error } = await deleteUserRoleBinding(user.id, binding.id);
      if (error || !data) {
        throw new Error(readProblemCode(error) || "USER_ROLE_BINDING_DELETE_FAILED");
      }
      setSuccessMessage(data.message || text.toasts.deleted);
      setErrorMessage("");
      await loadUsers();
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
        <div className="border border-accent/40 bg-accent/10 px-3 py-2 text-sm">
          <span className="text-muted">{text.dataScopeLabel}</span>
          <span>{dataScopeSummary || formatDepartmentScope(session)}</span>
        </div>
      </div>

      {!canAssign ? <p className="border border-white/10 bg-white/[0.04] px-3 py-2 text-sm text-muted">{text.readOnly}</p> : null}
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

      <Field label={text.searchLabel}>
        <Input
          onChange={(event) => setSearch(event.target.value)}
          placeholder={text.searchPlaceholder}
          value={search}
        />
      </Field>

      <div className="overflow-x-auto border border-white/10">
        <Table className="min-w-[900px]">
          <TableHeader>
            <TableRow>
              {[text.columns.name, text.columns.employeeId, text.columns.roles, text.columns.departments, text.columns.operations].map((label) => (
                <TableHead key={label}>
                  {label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.length > 0 ? (
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">
                    <Highlighted value={user.name} query={search} />
                  </TableCell>
                  <TableCell>
                    <Highlighted value={user.employeeId} query={search} />
                  </TableCell>
                  <TableCell>
                    <BindingChips bindings={user.roleBindings} query={search} />
                  </TableCell>
                  <TableCell>
                    {user.departments.length > 0 ? (
                      <span className="flex flex-wrap gap-2">
                        {user.departments.map((department) => (
                          <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={department.id}>
                            <Highlighted value={department.name} query={search} />
                          </span>
                        ))}
                      </span>
                    ) : (
                      <span className="text-muted">{text.emptyDepartment}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-2">
                      {canAssign && user.canAssign ? (
                        <Button onClick={() => void openAssignment(user)} type="button" variant="primary">
                          {text.actions.assign}
                        </Button>
                      ) : null}
                      {canDeleteBySession
                        ? user.roleBindings
                            .filter((binding) => binding.canDelete && !binding.guest)
                            .map((binding) => (
                              <Button key={binding.id} onClick={() => void removeBinding(user, binding)} type="button">
                                {text.actions.remove}
                              </Button>
                            ))
                        : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell className="py-8 text-center text-muted" colSpan={5}>
                  {text.emptyUsers}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {activeUser ? (
        <AssignmentPanel
          departments={departments}
          onAdd={addPendingBinding}
          onCancel={() => setActiveUser(null)}
          onDepartmentChange={setSelectedDepartmentID}
          onRoleChange={setSelectedRoleID}
          onSave={() => void savePendingBindings()}
          pending={pendingBindings}
          roles={roles}
          selectedDepartmentID={selectedDepartmentID}
          selectedRoleID={selectedRoleID}
          user={activeUser}
        />
      ) : null}
    </div>
  );
}

function BindingChips({ bindings, query }: { bindings: UserRoleBinding[]; query: string }) {
  return (
    <span className="flex flex-wrap gap-2">
      {bindings.map((binding) => (
        <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={binding.id}>
          <span>
            <Highlighted value={binding.role.label} query={query} />
          </span>
          <span className="text-muted"> · </span>
          <span>
            <Highlighted value={binding.department.name} query={query} />
          </span>
          {binding.guest ? <span className="ml-1 text-accent">{text.protectedGuest}</span> : null}
          {!binding.role.enabled ? <span className="ml-1 text-red-200">{text.disabledRole}</span> : null}
        </span>
      ))}
    </span>
  );
}

function AssignmentPanel({
  departments,
  onAdd,
  onCancel,
  onDepartmentChange,
  onRoleChange,
  onSave,
  pending,
  roles,
  selectedDepartmentID,
  selectedRoleID,
  user,
}: {
  departments: DepartmentOption[];
  onAdd: () => void;
  onCancel: () => void;
  onDepartmentChange: (value: string) => void;
  onRoleChange: (value: string) => void;
  onSave: () => void;
  pending: PendingBinding[];
  roles: AssignableRole[];
  selectedDepartmentID: string;
  selectedRoleID: string;
  user: ManagedUser;
}) {
  return (
    <aside aria-label={text.assignment.label} className="grid gap-4 border border-white/10 bg-white/[0.03] p-4" role="dialog">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="grid gap-1">
          <h2 className="text-xl font-semibold tracking-normal">{text.assignment.title}</h2>
          <p className="text-sm text-muted">
            {user.name} · {user.employeeId}
          </p>
        </div>
        <Button onClick={onCancel} type="button">
          {text.actions.cancel}
        </Button>
      </div>

      <section className="grid gap-2">
        <h3 className="text-sm font-semibold tracking-normal">{text.assignment.existing}</h3>
        <BindingChips bindings={user.roleBindings} query="" />
      </section>

      <Form className="md:grid-cols-[1fr_1fr_auto]" onSubmit={(event) => event.preventDefault()}>
        <Field label={text.assignment.role}>
          <Select onChange={(event) => onRoleChange(event.target.value)} value={selectedRoleID || roles[0]?.id || ""}>
            {roles.map((role) => (
              <option key={role.id} value={role.id}>
                {role.label}
                {role.attributeConditionSummary ? ` · ${role.attributeConditionSummary}` : ""}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={text.assignment.department}>
          <Select
            onChange={(event) => onDepartmentChange(event.target.value)}
            value={selectedDepartmentID || departments[0]?.id || ""}
          >
            {departments.map((department) => (
              <option key={department.id} value={department.id}>
                {department.name}
              </option>
            ))}
          </Select>
        </Field>
        <Button onClick={onAdd} type="button">
          {text.actions.addPending}
        </Button>
      </Form>

      <section className="grid gap-2">
        <h3 className="text-sm font-semibold tracking-normal">{text.assignment.pending}</h3>
        {pending.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {pending.map((binding) => (
              <span className="border border-accent/30 bg-accent/10 px-2 py-1 text-xs" key={`${binding.departmentId}-${binding.roleId}`}>
                {roleLabel(roles, binding.roleId)} · {departmentLabel(departments, binding.departmentId)}
              </span>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted">{text.assignment.noPending}</p>
        )}
      </section>

      <div>
        <Button onClick={onSave} type="button" variant="primary">
          {text.actions.save}
        </Button>
      </div>
    </aside>
  );
}

function Highlighted({ value, query }: { value: string; query: string }) {
  const needle = query.trim();
  if (!needle) {
    return <>{value}</>;
  }
  const index = value.toLowerCase().indexOf(needle.toLowerCase());
  if (index < 0) {
    return <>{value}</>;
  }
  return (
    <>
      {value.slice(0, index)}
      <mark className="bg-accent px-0.5 text-black">{value.slice(index, index + needle.length)}</mark>
      {value.slice(index + needle.length)}
    </>
  );
}

function roleLabel(roles: AssignableRole[], roleID: string) {
  return roles.find((role) => role.id === roleID)?.label ?? roleID;
}

function departmentLabel(departments: DepartmentOption[], departmentID: string) {
  return departments.find((department) => department.id === departmentID)?.name ?? departmentID;
}

function formatDepartmentScope(session: UsersSession) {
  if (session.dataScope.allDepartments) {
    return zhCN.session.workspace.allDepartments;
  }
  if (session.dataScope.departments.length === 0) {
    return zhCN.session.workspace.noDepartmentScope;
  }
  return session.dataScope.departments.map((department) => department.name).join("、");
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
