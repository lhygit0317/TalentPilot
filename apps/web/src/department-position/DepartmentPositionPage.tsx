import * as React from "react";
import { getDepartment, getPosition, listDepartments, listPositions } from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
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
        return;
      }
      setDepartmentDetail(data as DepartmentDetail);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.departmentDetail);
    }
  }

  async function handleOpenPosition(id: string) {
    try {
      const { data, error } = await getPosition(id);
      if (error || !data) {
        setErrorMessage(text.errors.positionDetail);
        return;
      }
      setPositionDetail(data as PositionDetail);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.positionDetail);
    }
  }

  const visibleDepartments = departments;
  const visiblePositions = positions;

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

      {activeTab === "departments" ? (
        <DepartmentTab
          departments={visibleDepartments}
          detail={departmentDetail}
          onOpen={(id) => void handleOpenDepartment(id)}
        />
      ) : (
        <PositionTab
          departmentFilter={departmentFilter}
          departments={departments}
          detail={positionDetail}
          onChannelChange={setChannelFilter}
          onDepartmentChange={setDepartmentFilter}
          onOpen={(id) => void handleOpenPosition(id)}
          onSearchChange={setSearch}
          onStatusChange={setStatusFilter}
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
  departments,
  detail,
  onOpen,
}: {
  departments: DepartmentListItem[];
  detail: DepartmentDetail | null;
  onOpen: (id: string) => void;
}) {
  return (
    <div className="grid gap-4">
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
                    {department.canGet ? (
                      <Button onClick={() => onOpen(department.id)} type="button">
                        {text.actions.view}
                      </Button>
                    ) : null}
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
  channelFilter,
  departmentFilter,
  departments,
  detail,
  onChannelChange,
  onDepartmentChange,
  onOpen,
  onSearchChange,
  onStatusChange,
  positions,
  search,
  statusFilter,
}: {
  channelFilter: string;
  departmentFilter: string;
  departments: DepartmentListItem[];
  detail: PositionDetail | null;
  onChannelChange: (value: string) => void;
  onDepartmentChange: (value: string) => void;
  onOpen: (id: string) => void;
  onSearchChange: (value: string) => void;
  onStatusChange: (value: string) => void;
  positions: PositionListItem[];
  search: string;
  statusFilter: string;
}) {
  return (
    <div className="grid gap-4">
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
                    {position.canGet ? (
                      <Button onClick={() => onOpen(position.id)} type="button">
                        {text.actions.view}
                      </Button>
                    ) : null}
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
