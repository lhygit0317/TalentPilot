export type ResumeChannel = "social" | "campus";

export type DepartmentSummary = {
  id: string;
  name: string;
};

export type ResumeListItem = {
  age?: number;
  canDelete: boolean;
  canGet: boolean;
  chan: string;
  currentDepartment: DepartmentSummary;
  id: string;
  keywords: string[];
  name: string;
  pos: string;
  school: string;
  source: string;
  sourceBy: string;
  yearsExp?: number;
};

export type ResumeListResponse = {
  availableChannels: string[];
  channelCounts: Record<string, number>;
  dataScopeSummary: string;
  items: ResumeListItem[];
  nextCursor: string;
};

export type ResumeProfile = {
  basic: Record<string, unknown>;
  certificates: string[];
  education: Array<Record<string, unknown>>;
  projects: Array<Record<string, unknown>>;
  rawTextRef: string;
  skills: string[];
  workExperience: Array<Record<string, unknown>>;
};

export type ResumeDetail = ResumeListItem & {
  createdAt: string;
  expired: boolean;
  profile: ResumeProfile;
};

export type ResumeLibrarySession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: DepartmentSummary[];
  };
  permissions: string[];
};
