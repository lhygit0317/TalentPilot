import * as React from "react";
import {
  generateInterviewQuestions,
  getJob,
  importResume,
  listPositions,
  listResumes,
  parseResumeMatch,
} from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { zhCN } from "../i18n/zh-CN";
import type { JobStatus, ResumeChannel, ResumeListItem, ResumeListResponse } from "../resume-library/types";
import type { InterviewQuestionGroup, InterviewQuestionResult, ParsePosition, ParseResult, ParseSession } from "./types";

const text = zhCN.resumeParse;
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

type ResumeParsePageProps = {
  session: ParseSession;
};

export function ResumeParsePage({ session }: ResumeParsePageProps) {
  const [channel, setChannel] = React.useState<ResumeChannel>(() => preferredChannel(session.dataScope.channels));
  const [sourceMode, setSourceMode] = React.useState<SourceMode>("library");
  const [list, setList] = React.useState<ResumeListResponse | null>(null);
  const [positions, setPositions] = React.useState<ParsePosition[]>([]);
  const [selectedResume, setSelectedResume] = React.useState<ResumeListItem | null>(null);
  const [selectedPositionId, setSelectedPositionId] = React.useState("");
  const [parseResult, setParseResult] = React.useState<ParseResult | null>(null);
  const [questions, setQuestions] = React.useState<InterviewQuestionResult | null>(null);
  const [activeQuestionType, setActiveQuestionType] = React.useState("professional");
  const [errorMessage, setErrorMessage] = React.useState("");
  const [successMessage, setSuccessMessage] = React.useState("");
  const [isParsing, setIsParsing] = React.useState(false);
  const [isGeneratingQuestions, setIsGeneratingQuestions] = React.useState(false);

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

  const loadPositions = React.useCallback(
    async (isCurrent: () => boolean = () => true) => {
      try {
        const { data, error } = await listPositions({ chan: channel, status: "on" });
        if (!isCurrent()) {
          return;
        }
        if (error || !data) {
          setErrorMessage(text.errors.positions);
          setPositions([]);
          setSelectedPositionId("");
          return;
        }
        const nextPositions = data.items ?? [];
        setPositions(nextPositions);
        setSelectedPositionId(nextPositions[0]?.id ?? "");
      } catch {
        if (isCurrent()) {
          setErrorMessage(text.errors.positions);
          setPositions([]);
          setSelectedPositionId("");
        }
      }
    },
    [channel],
  );

  React.useEffect(() => {
    let isCurrent = true;
    clearResultState();
    setSelectedResume(null);
    void loadResumes(() => isCurrent);
    void loadPositions(() => isCurrent);
    return () => {
      isCurrent = false;
    };
  }, [loadPositions, loadResumes]);

  function clearResultState() {
    setParseResult(null);
    setQuestions(null);
    setActiveQuestionType("professional");
  }

  function handleChannelChange(nextChannel: ResumeChannel) {
    if (nextChannel !== channel) {
      setChannel(nextChannel);
      setSuccessMessage("");
      setErrorMessage("");
    }
  }

  function handleSourceChange(nextMode: SourceMode) {
    if (nextMode !== sourceMode) {
      setSourceMode(nextMode);
      setSelectedResume(null);
      clearResultState();
      setSuccessMessage("");
      setErrorMessage("");
    }
  }

  function handleResumeSelect(resume: ResumeListItem) {
    setSelectedResume(resume);
    clearResultState();
    setSuccessMessage("");
    setErrorMessage("");
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
    const targetDepartmentId = resolveImportTargetDepartment(session);
    if (!targetDepartmentId) {
      setErrorMessage(text.errors.targetDepartmentRequired);
      setSuccessMessage("");
      return;
    }
    const body = new FormData();
    body.append("chan", channel);
    body.append("targetDepartmentId", targetDepartmentId);
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
      const department = session.dataScope.departments.find((item) => item.id === targetDepartmentId) ?? {
        id: targetDepartmentId,
        name: targetDepartmentId,
      };
      const importedResume: ResumeListItem = {
        id: imported?.resumeId || "",
        name: imported?.name || file.name,
        currentDepartment: department,
        pos: text.emptyValue,
        source: "导入",
        sourceBy: "",
        chan: channel,
        keywords: [],
        canDelete: false,
        canGet: true,
        school: "",
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

  async function handleParse() {
    if (!selectedResume || !selectedPositionId) {
      return;
    }
    setIsParsing(true);
    setErrorMessage("");
    setSuccessMessage("");
    clearResultState();
    try {
      const { data, error } = await parseResumeMatch({ resumeId: selectedResume.id, positionId: selectedPositionId });
      if (error || !data) {
        throw new Error(readProblemCode(error) || "MATCHING_PARSE_FAILED");
      }
      setParseResult(data);
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
    } finally {
      setIsParsing(false);
    }
  }

  async function handleGenerateQuestions() {
    if (!parseResult) {
      return;
    }
    setIsGeneratingQuestions(true);
    setErrorMessage("");
    try {
      const { data, error } = await generateInterviewQuestions({
        resumeId: parseResult.resume.id,
        positionId: parseResult.position.id,
        matchScore: parseResult.score.total,
      });
      if (error || !data) {
        throw new Error(readProblemCode(error) || "MATCHING_INTERVIEW_FAILED");
      }
      setQuestions(data);
      setActiveQuestionType(data.groups[0]?.type ?? "professional");
    } catch (error) {
      setErrorMessage(formatWorkflowError(error));
    } finally {
      setIsGeneratingQuestions(false);
    }
  }

  const authorizedChannels = authorizedResumeChannels(session.dataScope.channels);
  const resumes = list?.items ?? [];
  const selectedPosition = positions.find((position) => position.id === selectedPositionId);
  const canParse = Boolean(selectedResume && selectedPositionId && hasPermissions(session.permissions, ["Resume.Get", "Position.Get", "PositionResume.Create"]));

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
              <Field label={text.upload.label}>
                <Input onChange={handleImport} type="file" />
              </Field>
              {selectedResume ? <p className="text-sm text-accent">{text.selectedResume(selectedResume.name)}</p> : null}
            </div>
          )}

          <Field label={text.position.label}>
            <Select
              disabled={positions.length === 0}
              onChange={(event) => {
                setSelectedPositionId(event.target.value);
                clearResultState();
              }}
              value={selectedPositionId}
            >
              {positions.map((position) => (
                <option key={position.id} value={position.id}>
                  {formatPositionOption(position)}
                </option>
              ))}
            </Select>
          </Field>
          {positions.length === 0 ? <p className="text-sm text-muted">{text.empty.positions}</p> : null}

          <Button disabled={!canParse || isParsing} onClick={() => void handleParse()} type="button" variant="primary">
            {isParsing ? text.actions.parsing : text.actions.parse}
          </Button>
        </section>

        <section className="min-h-[460px] border border-white/10 p-4">
          {isParsing ? <ThinkingPanel label={text.result.thinking} /> : null}
          {!isParsing && parseResult ? (
            <ParseResultPanel
              onGenerateQuestions={() => void handleGenerateQuestions()}
              position={selectedPosition}
              questions={questions}
              result={parseResult}
              isGeneratingQuestions={isGeneratingQuestions}
              activeQuestionType={activeQuestionType}
              onQuestionTabChange={setActiveQuestionType}
            />
          ) : null}
          {!isParsing && !parseResult ? (
            <div className="grid h-full min-h-[420px] place-items-center text-center text-sm text-muted">
              <p>{text.result.empty}</p>
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
              {resume.source === "推荐" ? " · 推荐" : ""}
            </span>
          </span>
        </Button>
      ))}
    </div>
  );
}

function ParseResultPanel({
  activeQuestionType,
  isGeneratingQuestions,
  onGenerateQuestions,
  onQuestionTabChange,
  questions,
  result,
}: {
  activeQuestionType: string;
  isGeneratingQuestions: boolean;
  onGenerateQuestions: () => void;
  onQuestionTabChange: (type: string) => void;
  position?: ParsePosition;
  questions: InterviewQuestionResult | null;
  result: ParseResult;
}) {
  const activeGroup = questions?.groups.find((group) => group.type === activeQuestionType) ?? questions?.groups[0];
  return (
    <div className="grid gap-5">
      <header className="flex flex-wrap items-start justify-between gap-4 border-b border-white/10 pb-4">
        <div className="grid gap-2">
          <h2 className="text-xl font-semibold tracking-normal">{result.resume.name}</h2>
          <p className="text-sm text-muted">
            {shortChannelLabels[result.resume.chan] ?? result.resume.chan} · {result.position.name} ·{" "}
            {result.resume.currentDepartment.name}
          </p>
          <KeywordRow keywords={result.resume.keywords} />
        </div>
        <div className="grid min-w-32 place-items-center border border-white/10 p-3">
          <span className={scoreClassName(result.score.total)}>{result.score.total}</span>
          <span className="text-sm">{result.score.judgement}</span>
        </div>
      </header>

      <div className="grid gap-3 md:grid-cols-3">
        <ScoreBar label={text.result.skill} value={result.score.skill} />
        <ScoreBar label={text.result.experience} value={result.score.experience} />
        <ScoreBar label={text.result.implicit} value={result.score.implicit} />
      </div>

      <EvidenceSection title={text.result.keywords} items={result.evidence.keywords} />
      <EvidenceSection title={text.result.implicitTags} items={result.evidence.implicitTags} />
      <p className="border border-accent/30 bg-accent/10 px-3 py-2 text-sm">{result.evidence.analysis}</p>

      <div className="grid gap-3 border-t border-white/10 pt-4">
        <Button disabled={isGeneratingQuestions} onClick={onGenerateQuestions} type="button">
          {isGeneratingQuestions ? text.questions.thinking : text.actions.generateQuestions}
        </Button>
        {isGeneratingQuestions ? <ThinkingPanel label={text.questions.thinking} /> : null}
        {questions && activeGroup ? (
          <QuestionTabs activeType={activeQuestionType} group={activeGroup} groups={questions.groups} onChange={onQuestionTabChange} />
        ) : null}
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

function EvidenceSection({ title, items }: { title: string; items: Array<{ name: string; matched: boolean; w?: number }> }) {
  return (
    <section className="grid gap-2">
      <h3 className="text-sm font-semibold tracking-normal">{title}</h3>
      <div className="flex flex-wrap gap-2">
        {items.length > 0 ? (
          items.map((item) => (
            <span className={item.matched ? "border border-accent/40 bg-accent/10 px-2 py-1 text-sm" : "border border-white/10 bg-white/5 px-2 py-1 text-sm text-muted"} key={`${title}-${item.name}`}>
              {item.matched ? "✓ " : ""}
              {item.name}
              {typeof item.w === "number" ? ` ${item.w}` : ""}
            </span>
          ))
        ) : (
          <span className="text-sm text-muted">{text.emptyValue}</span>
        )}
      </div>
    </section>
  );
}

function QuestionTabs({
  activeType,
  group,
  groups,
  onChange,
}: {
  activeType: string;
  group: InterviewQuestionGroup;
  groups: InterviewQuestionGroup[];
  onChange: (type: string) => void;
}) {
  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap gap-2" role="tablist">
        {groups.map((item) => (
          <Button
            aria-selected={item.type === activeType}
            key={item.type}
            onClick={() => onChange(item.type)}
            role="tab"
            type="button"
            variant={item.type === activeType ? "primary" : "secondary"}
          >
            {questionTabLabel(item)}
          </Button>
        ))}
      </div>
      <div className="grid gap-2">
        {group.questions.map((question) => (
          <article className="grid gap-1 border border-white/10 p-3" key={`${group.type}-${question.order}`}>
            <h4 className="font-medium tracking-normal">
              {question.order}. {question.question}
            </h4>
            <p className="text-sm text-muted">{question.why}</p>
            <span className="w-fit border border-white/15 bg-white/5 px-2 py-1 text-xs">{question.difficulty}</span>
          </article>
        ))}
      </div>
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
  const result = channels.filter((channel): channel is ResumeChannel => channel === "social" || channel === "campus");
  return result.length > 0 ? result : ["social"];
}

function formatPositionOption(position: ParsePosition) {
  return `${position.department.name} · ${position.name}(${shortChannelLabels[position.chan] ?? position.chan})`;
}

function resolveImportTargetDepartment(session: ParseSession) {
  const departments = session.dataScope.departments.filter((department) => department.id !== "__system__");
  return departments.length === 1 ? departments[0].id : "";
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
  const { data, error } = await getJob(jobId);
  if (error || !data) {
    throw new Error(readProblemCode(error) || "JOB_ACCESS_DENIED");
  }
  return data;
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

function scoreClassName(score: number) {
  if (score >= 80) {
    return "text-4xl font-semibold text-accent";
  }
  if (score >= 65) {
    return "text-4xl font-semibold text-amber-200";
  }
  return "text-4xl font-semibold text-red-200";
}

function questionTabLabel(group: InterviewQuestionGroup) {
  if (group.type === "professional") {
    return text.questions.tabs.professional;
  }
  if (group.type === "manager") {
    return text.questions.tabs.manager;
  }
  if (group.type === "qualification") {
    return text.questions.tabs.qualification;
  }
  return group.label;
}
