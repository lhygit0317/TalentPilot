import type { DepartmentSummary, ResumeChannel } from "../resume-library/types";

export type ParseSession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: DepartmentSummary[];
  };
  permissions: string[];
};

export type ParsePosition = {
  id: string;
  name: string;
  department: DepartmentSummary;
  chan: ResumeChannel | string;
  level: string;
  status: "on" | "off" | string;
  keywordCount?: number;
  implicitTagCount?: number;
};

export type ParseResume = {
  chan: ResumeChannel | string;
  currentDepartment: DepartmentSummary;
  expBase?: number;
  id: string;
  keywords: string[];
  name: string;
  pos: string;
  source: string;
  sourceBy: string;
  traits?: string[];
};

export type MatchingImplicitTag = {
  name: string;
  w: number;
};

export type MatchEvidenceItem = {
  name: string;
  matched: boolean;
};

export type MatchWeightedEvidenceItem = MatchEvidenceItem & {
  w: number;
};

export type ParseResult = {
  id: string;
  resume: ParseResume;
  position: ParsePosition & {
    keywords: string[];
    implicitTags: MatchingImplicitTag[];
  };
  score: {
    total: number;
    skill: number;
    experience: number;
    implicit: number;
    judgement: string;
  };
  evidence: {
    keywords: MatchEvidenceItem[];
    implicitTags: MatchWeightedEvidenceItem[];
    analysis: string;
  };
  createdAt: string;
};

export type InterviewQuestion = {
  order: number;
  question: string;
  why: string;
  difficulty: string;
};

export type InterviewQuestionGroup = {
  type: string;
  label: string;
  questions: InterviewQuestion[];
};

export type InterviewQuestionResult = {
  groups: InterviewQuestionGroup[];
};
