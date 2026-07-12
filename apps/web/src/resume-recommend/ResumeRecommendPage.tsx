import * as React from "react";
import { getJob, importResume, listResumes, routeRecommendation, sendRecommendation } from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { zhCN } from "../i18n/zh-CN";
import type { JobStatus, ResumeListItem, ResumeListResponse } from "../resume-library/types";
import type { ResumeChannel, ResumeRecommendSession, RouteResult, RouteRow } from "./types";

const text = zhCN.resumeRecommend;
const maxImportBytes = 10 * 1024 * 1024;

const channelLabels: Record<ResumeChannel, string> = {
  campus: "校招 CAMPUS",
  social: "社招 SOCIAL",
};

const shortChannelLabels: Record<string, string> = {
  campus: "校招",
  social: "社招",
};

type SourceMode = "library" | "upload";

type ResumeRecommendPageProps = {
  session: ResumeRecommendSession;
};

export function ResumeRecommendPage({ session }: ResumeRecommendPageProps) {
  const [channel, setChannel] = React.useState<ResumeChannel>(() => preferredChannel(session.dataScope.channels));
  const [sourceMode, setSourceMode] = React.useState<SourceMode>("library");
  const [list, setList] = React.useState<ResumeListResponse | null>(null);
  const [selectedResume, setSelectedResume] = React.useState<ResumeListItem | null>(null);
  const [targetDepartmentId, setTargetDepartmentId] = React.useState("");
  const [routeResult, setRouteResult] = React.useState<RouteResult | null>(null);
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [isRouting, setIsRouting] = React.useState(false);
  const [sendingRouteKey, setSendingRouteKey] = React.useState("");

  const loadResumes = React.useCallback(
    async (isCurrent: () => boolean = () => true) => {
      try {
        const { data, error } = await listResumes({ chan: channel });
        if (!isCurrent()) {
          return;
        }
        if (error || !data) {
          setErrorMessage(text.errors.list);
          return;
        }
        setList(data);
      } catch {
        if (isCurrent()) {
          setErrorMessage(text.errors.list);
        }
      }
    },
    [channel],
  );

  React.useEffect(() => {
    let isCurrent = true;
    clearResultState();
    setSelectedResume(null);
    setSuccessMessage("");
    void loadResumes(() => isCurrent);
    return () => {
      isCurrent = false;
    };
  }, [loadResumes]);

  function clearResultState() {
    setRouteResult(null);
    setSendingRouteKey("");
  }

  function handleChannelChange(nextChannel: ResumeChannel) {
    if (nextChannel !== channel) {
      setChannel(nextChannel);
      setErrorMessage("");
      setSuccessMessage("");
    }
  }

  function handleSourceChange(nextMode: SourceMode) {
    if (nextMode !== sourceMode) {
      setSourceMode(nextMode);
      setSelectedResume(null);
      setErrorMessage("");
      setSuccessMessage("");
      clearResultState();
    }
  }

  function handleResumeSelect(resume: ResumeListItem) {
    setSelectedResume(resume);
    setErrorMessage("");
    setSuccessMessage("");
    clearResultState();
  }

  async function handleImport(event: React.ChangeEvent<HTMLInputElement>) {
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
    const body = new FormData();
    body.append("chan", channel);
    body.append("targetDepartmentId", targetDepartment.id);
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
      const imported = job.results?.find((result) => result.status === "succeeded");
      const importedResume: ResumeListItem = {
        id: imported?.resumeId || "",
        name: imported?.name || file.name,
        age: undefined,
        school: "",
        yearsExp: undefined,
        currentDepartment: targetDepartment,
        pos: text.emptyValue,
        source: "导入",
        sourceBy: "",
        chan: channel,
        keywords: [],
        canDelete: false,
        canGet: true,
      };
      setSelectedResume(importedResume);
      setSuccessMessage(text.toasts.singleImported(importedResume.name));
      setErrorMessage("");
      clearResultState();
      await loadResumes();
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
      setSuccessMessage("");
    }
  }

  async function handleRoute() {
    if (!selectedResume) {
      return;
    }
    setIsRouting(true);
    setErrorMessage("");
    setSuccessMessage("");
    clearResultState();
    try {
      const { data, error } = await routeRecommendation({ resumeId: selectedResume.id });
      if (error || !data) {
        throw new Error(readProblemCode(error) || "RECOMMENDATION_ROUTE_FAILED");
      }
      setRouteResult(data);
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
    } finally {
      setIsRouting(false);
    }
  }

  async function handleSend(route: RouteRow) {
    if (!selectedResume) {
      return;
    }
    const routeKey = routeKeyFor(route);
    setSendingRouteKey(routeKey);
    setErrorMessage("");
    setSuccessMessage("");
    try {
      const { data, error } = await sendRecommendation({
        resumeId: selectedResume.id,
        departmentId: route.department.id,
        positionId: route.position.id,
      });
      if (error || !data) {
        throw new Error(readProblemCode(error) || "RECOMMENDATION_SEND_FAILED");
      }
      setSuccessMessage(data.message);
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
    } finally {
      setSendingRouteKey("");
    }
  }

  const authorizedChannels = authorizedResumeChannels(session.dataScope.channels);
  const resumes = list?.items ?? [];
  const canRoute = Boolean(selectedResume && hasPermissions(session.permissions, ["Resume.Get"]));
  const canSend = hasPermissions(session.permissions, [
    "Resume.Get",
    "Resume.Create",
    "DepartmentResume.Create",
    "PositionResume.Create",
    "Notification.Create",
  ]);
  const importDepartments = concreteDepartments(session);
  const needsTargetDepartment = importDepartments.length > 1;

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-normal">{text.title}</h1>
          <p className="text-sm text-muted">{text.subtitle}</p>
        </div>
        <div className="flex flex-wrap gap-2" aria-label={text.channelNavLabel}>
          {authorizedChannels.map((item) => (
            <Button
              aria-pressed={item === channel}
              key={item}
              onClick={() => handleChannelChange(item)}
              type="button"
              variant={item === channel ? "primary" : "secondary"}
            >
              {channelLabels[item]}
            </Button>
          ))}
        </div>
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

      <div className="grid gap-6 lg:grid-cols-[minmax(320px,420px)_1fr]">
        <section className="grid gap-4 border border-white/10 p-4">
          <div className="grid grid-cols-2 gap-2">
            <Button
              aria-pressed={sourceMode === "library"}
              onClick={() => handleSourceChange("library")}
              type="button"
              variant={sourceMode === "library" ? "primary" : "secondary"}
            >
              {text.source.library}
            </Button>
            <Button
              aria-pressed={sourceMode === "upload"}
              onClick={() => handleSourceChange("upload")}
              type="button"
              variant={sourceMode === "upload" ? "primary" : "secondary"}
            >
              {text.source.upload}
            </Button>
          </div>

          {sourceMode === "library" ? (
            <ResumePicker resumes={resumes} selectedResumeId={selectedResume?.id ?? ""} onSelect={handleResumeSelect} />
          ) : (
            <div className="grid gap-3 border border-dashed border-white/20 p-4">
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
              <Field label={text.upload.label}>
                <Input onChange={handleImport} type="file" />
              </Field>
              {selectedResume ? <p className="text-sm text-accent">{text.selectedResume(selectedResume.name)}</p> : null}
            </div>
          )}

          <Button disabled={!canRoute || isRouting} onClick={() => void handleRoute()} type="button" variant="primary">
            {isRouting ? text.actions.routing : text.actions.route}
          </Button>
        </section>

        <section className="min-h-[460px] border border-white/10 p-4">
          {isRouting ? <ThinkingPanel label={text.result.thinking} /> : null}
          {!isRouting && routeResult ? (
            <RouteResultPanel
              canSend={canSend}
              result={routeResult}
              sendingRouteKey={sendingRouteKey}
              onSend={(route) => void handleSend(route)}
            />
          ) : null}
          {!isRouting && !routeResult ? (
            <div className="grid h-full min-h-[420px] place-items-center text-center text-sm text-muted">
              <div className="grid gap-1">
                <p>{text.result.emptyTitle}</p>
                <p>{text.result.emptyDescription}</p>
              </div>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}

function ResumePicker({
  resumes,
  selectedResumeId,
  onSelect,
}: {
  resumes: ResumeListItem[];
  selectedResumeId: string;
  onSelect: (resume: ResumeListItem) => void;
}) {
  if (resumes.length === 0) {
    return <p className="py-8 text-center text-sm text-muted">{text.empty.resumes}</p>;
  }
  return (
    <div className="grid gap-2">
      {resumes.map((resume) => (
        <Button
          aria-pressed={resume.id === selectedResumeId}
          className="justify-start text-left"
          key={resume.id}
          onClick={() => onSelect(resume)}
          type="button"
          variant={resume.id === selectedResumeId ? "primary" : "secondary"}
        >
          <span className="grid gap-1">
            <span>{resume.name}</span>
            <span className="text-xs opacity-80">
              {resume.pos || text.emptyValue} · {resume.currentDepartment.name}
            </span>
          </span>
        </Button>
      ))}
    </div>
  );
}

function RouteResultPanel({
  canSend,
  result,
  sendingRouteKey,
  onSend,
}: {
  canSend: boolean;
  result: RouteResult;
  sendingRouteKey: string;
  onSend: (route: RouteRow) => void;
}) {
  if (result.routes.length === 0) {
    return (
      <div className="grid h-full min-h-[420px] place-items-center text-center text-sm text-muted">
        <div className="grid gap-1">
          <p>{text.result.noRoutesTitle}</p>
          <p>{text.result.noRoutesDescription}</p>
        </div>
      </div>
    );
  }
  return (
    <div className="grid gap-4">
      <header className="grid gap-2 border-b border-white/10 pb-4">
        <h2 className="text-xl font-semibold tracking-normal">{result.resume.name}</h2>
        <p className="text-sm text-muted">
          {shortChannelLabels[result.resume.chan] ?? result.resume.chan} · {result.resume.pos || text.emptyValue} ·{" "}
          {result.resume.currentDepartment.name}
        </p>
        <KeywordRow keywords={result.resume.keywords} />
      </header>

      <div className="grid gap-3">
        {result.routes.map((route) => {
          const routeKey = routeKeyFor(route);
          return (
            <article className="grid gap-4 border border-white/10 bg-white/[0.03] p-4" key={routeKey}>
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="grid gap-2">
                  {route.best ? <span className="w-fit border border-accent/40 bg-accent/10 px-2 py-1 text-xs">最佳去向</span> : null}
                  <h3 className="text-lg font-semibold tracking-normal">{route.department.name}</h3>
                  <p className="text-sm text-muted">
                    {route.position.name} · {route.position.level || text.emptyValue}
                  </p>
                </div>
                <div className="grid min-w-28 place-items-center border border-white/10 p-3">
                  <span className={scoreClassName(route.score.total)}>{route.score.total}</span>
                  <span className="text-sm">{route.score.judgement}</span>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-3">
                <ScoreBar label={text.result.skill} value={route.score.skill} />
                <ScoreBar label={text.result.experience} value={route.score.experience} />
                <ScoreBar label={text.result.implicit} value={route.score.implicit} />
              </div>

              <div className="grid gap-2 text-sm">
                <ContactGroup label={text.contacts.hrbps} names={route.contacts.hrbps} />
                <ContactGroup label={text.contacts.managers} names={route.contacts.managers} />
                <ContactGroup label={text.contacts.trainees} names={route.contacts.trainees} />
              </div>

              <Button
                disabled={!canSend || sendingRouteKey === routeKey}
                onClick={() => onSend(route)}
                type="button"
                variant={route.best ? "primary" : "secondary"}
              >
                {sendingRouteKey === routeKey ? text.actions.sending : text.actions.send}
              </Button>
            </article>
          );
        })}
      </div>
    </div>
  );
}

function KeywordRow({ keywords }: { keywords: string[] }) {
  if (keywords.length === 0) {
    return <span className="text-sm text-muted">{text.emptyValue}</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {keywords.map((keyword) => (
        <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={keyword}>
          {keyword}
        </span>
      ))}
    </div>
  );
}

function ScoreBar({ label, value }: { label: string; value: number }) {
  return (
    <div className="grid gap-2 border border-white/10 p-3">
      <div className="flex items-center justify-between gap-3 text-sm">
        <span>{label}</span>
        <span>{value}</span>
      </div>
      <div className="h-2 bg-white/10">
        <div className="h-full bg-accent" style={{ width: `${Math.max(0, Math.min(value, 100))}%` }} />
      </div>
    </div>
  );
}

function ContactGroup({ label, names }: { label: string; names: string[] }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-muted">{label}</span>
      {names.length > 0 ? (
        names.map((name) => (
          <span className="border border-white/15 bg-white/5 px-2 py-1 text-xs" key={`${label}-${name}`}>
            {name}
          </span>
        ))
      ) : (
        <span>{text.emptyValue}</span>
      )}
    </div>
  );
}

function ThinkingPanel({ label }: { label: string }) {
  return <div className="grid min-h-40 place-items-center border border-white/10 bg-white/[0.03] text-sm text-muted">{label}</div>;
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

function authorizedResumeChannels(channels: string[]): ResumeChannel[] {
  const result = channels.filter((item): item is ResumeChannel => item === "social" || item === "campus");
  return result.length > 0 ? result : ["social"];
}

function concreteDepartments(session: ResumeRecommendSession) {
  return session.dataScope.departments.filter((department) => department.id !== "__system__");
}

function resolveImportTargetDepartment(session: ResumeRecommendSession, selectedDepartmentId: string) {
  const departments = concreteDepartments(session);
  if (departments.length === 1) {
    return departments[0];
  }
  return departments.find((department) => department.id === selectedDepartmentId) ?? null;
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
  return text.errors.codes[code as keyof typeof text.errors.codes] ?? text.errors.generic;
}

function hasPermissions(permissions: string[], required: string[]) {
  return required.every((permission) => permissions.includes(permission));
}

function routeKeyFor(route: RouteRow) {
  return `${route.department.id}:${route.position.id}`;
}

function scoreClassName(score: number) {
  if (score >= 80) {
    return "text-4xl font-semibold text-accent";
  }
  if (score >= 65) {
    return "text-4xl font-semibold text-amber-200";
  }
  return "text-4xl font-semibold text-red-200";
}
