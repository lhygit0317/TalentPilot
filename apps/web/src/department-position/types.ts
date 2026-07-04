export type DepartmentPositionSession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: Array<{ id: string; name: string }>;
  };
  permissions: string[];
};

export type DepartmentListItem = {
  id: string;
  name: string;
  positionCount: number;
  resumeCount: number;
  updatedAt: string;
  canGet: boolean;
  canUpdate: boolean;
  canDelete: boolean;
};

export type DepartmentDetail = DepartmentListItem & {
  positions: PositionSummary[];
};

export type PositionSummary = {
  id: string;
  name: string;
  chan: string;
  level: string;
  status: string;
};

export type PositionListItem = {
  id: string;
  name: string;
  department: { id: string; name: string };
  chan: string;
  level: string;
  status: string;
  keywordCount: number;
  implicitTagCount: number;
  updatedAt: string;
  canGet: boolean;
  canUpdate: boolean;
  canDelete: boolean;
};

export type PositionDetail = PositionListItem & {
  duties: string[];
  must: string[];
  keywords: string[];
  implicitTags: Array<{ name: string; w: number }>;
};
