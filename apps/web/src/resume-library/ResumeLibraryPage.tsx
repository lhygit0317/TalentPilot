import * as React from "react";
import { getResume, listResumes } from "../api/client";
import { Button } from "../components/ui/button";
import { Field } from "../components/ui/form";
import { Input } from "../components/ui/input";
import { zhCN } from "../i18n/zh-CN";
import { highlightLiteral } from "./highlight";
import type { ResumeChannel, ResumeDetail, ResumeLibrarySession, ResumeListItem, ResumeListResponse } from "./types";

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

type ResumeLibraryPageProps = {
  session: ResumeLibrarySession;
};

export function ResumeLibraryPage({ session }: ResumeLibraryPageProps) {
  const [channel, setChannel] = React.useState<ResumeChannel>(() => preferredChannel(session.dataScope.channels));
  const [search, setSearch] = React.useState("");
  const [list, setList] = React.useState<ResumeListResponse | null>(null);
  const [detail, setDetail] = React.useState<ResumeDetail | null>(null);
  const [errorMessage, setErrorMessage] = React.useState("");

  React.useEffect(() => {
    let isCurrent = true;

    async function loadResumes() {
      const query = { chan: channel, search: search.trim() || undefined };

      try {
        const { data, error } = await listResumes(query);
        if (!isCurrent) {
          return;
        }
        if (error || !data) {
          setErrorMessage(text.errors.list);
          return;
        }
        setList(data);
        setErrorMessage("");
      } catch {
        if (isCurrent) {
          setErrorMessage(text.errors.list);
        }
      }
    }

    void loadResumes();

    return () => {
      isCurrent = false;
    };
  }, [channel, search]);

  async function handleOpenDetail(resumeId: string) {
    try {
      const { data, error } = await getResume(resumeId);
      if (error || !data) {
        setErrorMessage(text.errors.detail);
        return;
      }
      setDetail(data);
      setErrorMessage("");
    } catch {
      setErrorMessage(text.errors.detail);
    }
  }

  const availableChannels = list?.availableChannels.filter(isResumeChannel) ?? [channel];
  const dataScopeSummary = list?.dataScopeSummary || session.dataScope.departments.map((department) => department.name).join("、");
  const rows = list?.items ?? [];
  const emptyMessage = search.trim() ? text.empty.search : text.empty.channel;

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

      {errorMessage ? (
        <p className="border border-red-400/30 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">
          {errorMessage}
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
                        <Button disabled type="button">
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
