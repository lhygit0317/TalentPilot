export type ResumeChannel = "social" | "campus";

export type DepartmentSummary = {
  id: string;
  name: string;
};

export type ResumeRecommendSession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: DepartmentSummary[];
  };
  permissions: string[];
};

export type ResumeListItem = {
  chan: string;
  currentDepartment: DepartmentSummary;
  id: string;
  keywords: string[];
  name: string;
  pos: string;
  source: string;
  sourceBy: string;
};

export type RouteScore = {
  experience: number;
  implicit: number;
  judgement: string;
  skill: number;
  total: number;
};

export type RouteRow = {
  best: boolean;
  contacts: {
    hrbps: string[];
    managers: string[];
    trainees: string[];
  };
  department: DepartmentSummary;
  position: {
    chan: string;
    id: string;
    level: string;
    name: string;
  };
  score: RouteScore;
};

export type RouteResult = {
  createdAt: string;
  resume: {
    chan: string;
    currentDepartment: DepartmentSummary;
    id: string;
    keywords: string[];
    name: string;
    pos: string;
  };
  routes: RouteRow[];
};

export type SendRecommendationResult = {
  candidateName: string;
  department: DepartmentSummary;
  message: string;
  notifiedCount: number;
  position: {
    id: string;
    name: string;
  };
  resumeId: string;
  reusedExistingCopy: boolean;
  selfNotificationRead: boolean;
  sourceResumeId: string;
};
