import * as React from "react";
import { deleteResume, getJob, getResume, importResume, importResumesBatch, listResumes } from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { zhCN } from "../i18n/zh-CN";
import { highlightLiteral } from "./highlight";
import type {
  JobStatus,
  ResumeChannel,
  ResumeDetail,
  ResumeLibrarySession,
  ResumeListItem,
  ResumeListResponse,
} from "./types";

const text = zhCN.resumeLibrary;

const channelLabels: Record<ResumeChannel, string> = {
  campus: zhCN.session.workspace.channels.campus,
  social: zhCN.session.workspace.channels.social,
};

const columnLabels = [
  text.columns.candidate,
  text.columns.age,
  text.columns.school,
  text.columns.yearsExp,
  text.columns.currentDepartment,
  text.columns.position,
  text.columns.source,
  text.columns.keywords,
  text.columns.operations,
];

const maxImportBytes = 10 * 1024 * 1024;

type ResumeLibraryPageProps = {
  session: ResumeLibrarySession;
};

export function ResumeLibraryPage({ session }: ResumeLibraryPageProps) {
  const [channel, setChannel] = React.useState<ResumeChannel>(() => preferredChannel(session.dataScope.channels));
  const [search, setSearch] = React.useState("");
  const [list, setList] = React.useState<ResumeListResponse | null>(null);
  const [detail, setDetail] = React.useState<ResumeDetail | null>(null);
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [targetDepartmentId, setTargetDepartmentId] = React.useState("");

  const loadResumes = React.useCallback(
    async (isCurrent: () => boolean = () => true) => {
      const query = { chan: channel, search: search.trim() || undefined };

      try {
        const { data, error } = await listResumes(query);
        if (!isCurrent()) {
          return;
        }
        if (error || !data) {
          setErrorMessage(text.errors.list);
          return;
        }
        setList(data);
        setErrorMessage("");
      } catch {
        if (isCurrent()) {
          setErrorMessage(text.errors.list);
        }
      }
    },
    [channel, search],
  );

  React.useEffect(() => {
    let isCurrent = true;

    void loadResumes(() => isCurrent);

    return () => {
      isCurrent = false;
    };
  }, [loadResumes]);

  async function handleOpenDetail(resumeId: string) {
    try {
      const { data, error } = await getResume(resumeId);
      if (error || !data) {
        setErrorMessage(text.errors.detail);
        setSuccessMessage("");
        return;
      }
      setDetail(data);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.detail);
      setSuccessMessage("");
    }
  }

  async function handleSingleImport(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = "";
    if (!file) {
      return;
    }

    const validationError = validatePDF(file);
    if (validationError) {
      setErrorMessage(validationError);
      setSuccessMessage("");
      return;
    }

    const targetDepartment = resolveImportTargetDepartment(session, targetDepartmentId);
    if (!targetDepartment) {
      setErrorMessage(text.errors.targetDepartmentRequired);
      setSuccessMessage("");
      return;
    }

    const body = buildImportFormData(channel, targetDepartment);
    body.append("file", file);

    try {
      const { data, error } = await importResume(body);
      if (error || !data) {
        throw new Error(readProblemCode(error) || "RESUME_IMPORT_FAILED");
      }
      const job = await waitForJob(readJobId(data));
      if (job.status === "failed") {
        throw new Error(readJobErrorCode(job));
      }
      const importedName = job.results?.find((result) => result.status === "succeeded")?.name || file.name;
      setSuccessMessage(text.toasts.singleImported(importedName));
      setErrorMessage("");
      await loadResumes();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleBatchImport(event: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = "";
    if (files.length === 0) {
      return;
    }

    const validationError = files.map(validatePDF).find(Boolean);
    if (validationError) {
      setErrorMessage(validationError);
      setSuccessMessage("");
      return;
    }

    const targetDepartment = resolveImportTargetDepartment(session, targetDepartmentId);
    if (!targetDepartment) {
      setErrorMessage(text.errors.targetDepartmentRequired);
      setSuccessMessage("");
      return;
    }

    const body = buildImportFormData(channel, targetDepartment);
    for (const file of files) {
      body.append("files", file);
    }

    try {
      const { data, error } = await importResumesBatch(body);
      if (error || !data) {
        throw new Error(readProblemCode(error) || "RESUME_IMPORT_FAILED");
      }
      const job = await waitForJob(readJobId(data));
      if (job.status === "failed" && job.summary.succeeded === 0) {
        throw new Error(readJobErrorCode(job));
      }
      setSuccessMessage(text.toasts.batchImported(job.summary.succeeded, channelLabels[channel]));
      setErrorMessage("");
      await loadResumes();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleDeleteResume(resumeId: string) {
    try {
      const { error } = await deleteResume(resumeId);
      if (error) {
        throw new Error(readProblemCode(error) || "RESUME_DELETE_FAILED");
      }
      setSuccessMessage(text.toasts.deleted);
      setErrorMessage("");
      await loadResumes();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  const availableChannels = list?.availableChannels.filter(isResumeChannel) ?? [channel];
  const dataScopeSummary = list?.dataScopeSummary || session.dataScope.departments.map((department) => department.name).join("、");
  const rows = list?.items ?? [];
  const emptyMessage = search.trim() ? text.empty.search : text.empty.channel;
  const canImport = hasPermissions(session.permissions, ["Resume.Create", "DepartmentResume.Create"]);
  const importDepartments = concreteDepartments(session);
  const needsTargetDepartment = importDepartments.length !== 1;

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-normal">{text.title}</h1>
          <p className="text-sm text-muted">{text.subtitle}</p>
        </div>
        <div className="border border-accent/40 bg-accent/10 px-3 py-2 text-sm">
          <span className="text-muted">{text.dataScopeLabel}</span>
          <span>{dataScopeSummary || text.noDataScope}</span>
        </div>
      </div>

      <div className="flex flex-wrap items-end justify-between gap-4">
        <div aria-label={text.channelNavLabel} className="flex flex-wrap gap-2">
          {availableChannels.map((availableChannel) => (
            <Button
              aria-pressed={availableChannel === channel}
              key={availableChannel}
              onClick={() => setChannel(availableChannel)}
              type="button"
              variant={availableChannel === channel ? "primary" : "secondary"}
            >
              {channelLabels[availableChannel]} {list?.channelCounts[availableChannel] ?? 0}
            </Button>
          ))}
        </div>

        <Field className="min-w-64" label={text.searchLabel}>
          <Input
            onChange={(event) => setSearch(event.target.value)}
            placeholder={text.searchPlaceholder}
            type="search"
            value={search}
          />
        </Field>
      </div>

      {canImport ? (
        <div className="grid gap-3 border border-white/10 p-3 md:grid-cols-3">
          {needsTargetDepartment ? (
            <Field label={text.import.targetDepartmentLabel}>
              <Select onChange={(event) => setTargetDepartmentId(event.target.value)} value={targetDepartmentId}>
                <option value="">{text.import.targetDepartmentPlaceholder}</option>
                {importDepartments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </Select>
            </Field>
          ) : null}
          <Field label={text.import.singleLabel}>
            <Input onChange={handleSingleImport} type="file" />
          </Field>
          <Field label={text.import.batchLabel}>
            <Input multiple onChange={handleBatchImport} type="file" />
          </Field>
        </div>
      ) : null}

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

      <div className="overflow-x-auto border border-white/10">
        <table className="w-full min-w-[920px] border-collapse text-left text-sm">
          <thead className="bg-white/[0.04] text-muted">
            <tr>
              {columnLabels.map((label) => (
                <th className="border-b border-white/10 px-3 py-3 font-medium" key={label} scope="col">
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.length > 0 ? (
              rows.map((resume) => (
                <tr className="border-b border-white/10 last:border-0" key={resume.id}>
                  <td className="px-3 py-3 font-medium">{highlightLiteral(resume.name, search)}</td>
                  <td className="px-3 py-3">{formatOptionalNumber(resume.age)}</td>
                  <td className="px-3 py-3">{resume.school || text.emptyValue}</td>
                  <td className="px-3 py-3">{formatOptionalNumber(resume.yearsExp)}</td>
                  <td className="px-3 py-3">
                    <SplitText value={resume.currentDepartment.name || text.emptyValue} />
                  </td>
                  <td className="px-3 py-3">{highlightLiteral(resume.pos || text.emptyValue, search)}</td>
                  <td className="px-3 py-3">{formatSource(resume)}</td>
                  <td className="px-3 py-3">
                    <KeywordList keywords={resume.keywords} search={search} />
                  </td>
                  <td className="px-3 py-3">
                    <div className="flex flex-wrap gap-2">
                      {resume.canGet ? (
                        <Button onClick={() => void handleOpenDetail(resume.id)} type="button">
                          {text.actions.viewDetail}
                        </Button>
                      ) : null}
                      {resume.canDelete ? (
                        <Button onClick={() => void handleDeleteResume(resume.id)} type="button">
                          {text.actions.delete}
                        </Button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-3 py-8 text-center text-muted" colSpan={columnLabels.length}>
                  {emptyMessage}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {detail ? <ResumeDetailPanel detail={detail} /> : null}
    </div>
  );
}

function KeywordList({ keywords, search }: { keywords: string[]; search: string }) {
  if (keywords.length === 0) {
    return <span className="text-muted">{text.emptyValue}</span>;
  }

  return (
    <div className="flex flex-wrap gap-1">
      {keywords.map((keyword) => (
        <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={keyword}>
          {highlightLiteralKeyword(keyword, search)}
        </span>
      ))}
    </div>
  );
}

function SplitText({ value }: { value: string }) {
  if (value.length < 2) {
    return value;
  }

  const midpoint = Math.floor(value.length / 2);
  return (
    <>
      <span>{value.slice(0, midpoint)}</span>
      <span>{value.slice(midpoint)}</span>
    </>
  );
}

function highlightLiteralKeyword(value: string, query: string): React.ReactNode {
  const trimmedQuery = query.trim();
  if (!trimmedQuery) {
    return value;
  }

  const index = value.toLocaleLowerCase().indexOf(trimmedQuery.toLocaleLowerCase());
  if (index < 0) {
    return value;
  }

  return (
    <>
      {value.slice(0, index)}
      <mark>
        <SplitText value={value.slice(index, index + trimmedQuery.length)} />
      </mark>
      {value.slice(index + trimmedQuery.length)}
    </>
  );
}

function ResumeDetailPanel({ detail }: { detail: ResumeDetail }) {
  return (
    <aside aria-label={text.detail.label} className="grid gap-4 border border-white/10 bg-white/[0.03] p-4">
      <div className="grid gap-2">
        <h2 className="text-xl font-semibold tracking-normal">{detail.name}</h2>
        <dl className="grid gap-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <DetailField label={text.columns.age} value={formatOptionalNumber(detail.age)} />
          <DetailField label={text.columns.school} value={detail.school || text.emptyValue} />
          <DetailField label={text.columns.yearsExp} value={formatOptionalNumber(detail.yearsExp)} />
          <DetailField label={text.columns.currentDepartment} value={detail.currentDepartment.name || text.emptyValue} />
          <DetailField label={text.columns.position} value={detail.pos || text.emptyValue} />
          <DetailField label={text.columns.source} value={formatSource(detail)} />
          <DetailField label={text.detail.createdAt} value={formatDate(detail.createdAt)} />
          <DetailField label={text.detail.expired} value={detail.expired ? text.detail.yes : text.detail.no} />
        </dl>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <ProfileSection title={text.detail.sections.basic} value={detail.profile.basic} />
        <ProfileSection title={text.detail.sections.education} value={detail.profile.education} />
        <ProfileSection title={text.detail.sections.workExperience} value={detail.profile.workExperience} />
        <ProfileSection title={text.detail.sections.projects} value={detail.profile.projects} />
        <ProfileSection title={text.detail.sections.skills} value={detail.profile.skills} />
        <ProfileSection title={text.detail.sections.certificates} value={detail.profile.certificates} />
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

function ProfileSection({ title, value }: { title: string; value: unknown }) {
  return (
    <section className="grid gap-2 border border-white/10 p-3">
      <h3 className="text-sm font-semibold tracking-normal">{title}</h3>
      {isEmptyProfileValue(value) ? (
        <p className="text-sm text-muted">{text.emptyValue}</p>
      ) : (
        <pre className="whitespace-pre-wrap break-words text-sm text-muted">{formatProfileValue(value)}</pre>
      )}
    </section>
  );
}

function preferredChannel(channels: string[]): ResumeChannel {
  if (channels.includes("social")) {
    return "social";
  }
  if (channels.includes("campus")) {
    return "campus";
  }
  return "social";
}

function validatePDF(file: File) {
  if (file.size > maxImportBytes) {
    return text.errors.fileTooLarge;
  }
  if (file.type !== "application/pdf" && !file.name.toLocaleLowerCase().endsWith(".pdf")) {
    return text.errors.unsupportedType;
  }
  return "";
}

function buildImportFormData(channel: ResumeChannel, targetDepartmentId: string) {
  const body = new FormData();
  body.append("chan", channel);
  body.append("targetDepartmentId", targetDepartmentId);

  return body;
}

function resolveImportTargetDepartment(session: ResumeLibrarySession, selectedDepartmentId: string) {
  const departments = concreteDepartments(session);
  if (departments.length === 1) {
    return departments[0].id;
  }
  if (departments.some((department) => department.id === selectedDepartmentId)) {
    return selectedDepartmentId;
  }
  return "";
}

function concreteDepartments(session: ResumeLibrarySession) {
  return session.dataScope.departments.filter((department) => department.id !== "__system__");
}

async function waitForJob(jobId: string): Promise<JobStatus> {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const { data, error } = await getJob(jobId);
    if (error || !data) {
      throw new Error(readProblemCode(error) || "JOB_ACCESS_DENIED");
    }

    if (data.status === "succeeded" || data.status === "failed") {
      return data;
    }

    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }

  throw new Error("JOB_TIMEOUT");
}

function readJobId(data: { id?: string; jobId?: string }) {
  return data.jobId || data.id || "";
}

function readJobErrorCode(job: JobStatus) {
  return job.results?.find((result) => result.errorCode)?.errorCode || "RESUME_IMPORT_FAILED";
}

function readProblemCode(error: unknown) {
  if (error && typeof error === "object" && "code" in error && typeof error.code === "string") {
    return error.code;
  }
  return "";
}

function formatWorkflowError(error: unknown) {
  const code = error instanceof Error ? error.message : "";
  return text.errors.codes[code as keyof typeof text.errors.codes] ?? text.errors.importFailed;
}

function hasPermissions(permissions: string[], required: string[]) {
  return required.every((permission) => permissions.includes(permission));
}

function isResumeChannel(value: string): value is ResumeChannel {
  return value === "social" || value === "campus";
}

function formatOptionalNumber(value: number | undefined) {
  return typeof value === "number" ? String(value) : text.emptyValue;
}

function formatDate(value: string) {
  if (!value) {
    return text.emptyValue;
  }
  return value.slice(0, 10);
}

function formatSource(resume: Pick<ResumeListItem, "source" | "sourceBy">) {
  if (!resume.sourceBy) {
    return text.unknownSource;
  }
  return `${resume.sourceBy}${resume.source}`;
}

function isEmptyProfileValue(value: unknown) {
  if (Array.isArray(value)) {
    return value.length === 0;
  }
  if (value && typeof value === "object") {
    return Object.keys(value).length === 0;
  }
  return !value;
}

function formatProfileValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map((item) => formatProfileValue(item)).join("\n");
  }
  if (value && typeof value === "object") {
    return Object.entries(value)
      .map(([key, entryValue]) => `${key}: ${String(entryValue)}`)
      .join("\n");
  }
  return String(value);
}
