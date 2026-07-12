export type UsersSession = {
  dataScope: {
    allDepartments: boolean;
    channels: string[];
    departments: Array<{ id: string; name: string }>;
  };
  permissions: string[];
};

export type UserDepartmentSummary = {
  id: string;
  name: string;
  system: boolean;
};

export type UserRoleSummary = {
  id: string;
  label: string;
  isSystem: boolean;
  enabled: boolean;
};

export type UserRoleBinding = {
  id: string;
  role: UserRoleSummary;
  department: UserDepartmentSummary;
  guest: boolean;
  createdAt: string;
  createdBy: string;
  canDelete: boolean;
};

export type ManagedUser = {
  id: string;
  employeeId: string;
  name: string;
  departments: UserDepartmentSummary[];
  roleBindings: UserRoleBinding[];
  roleSummary: string;
  guestOnly: boolean;
  canAssign: boolean;
};

export type AssignableRole = {
  id: string;
  label: string;
  description: string;
  isSystem: boolean;
  supportsSystemDepartment: boolean;
  attributeConditionSummary: string;
};

export type DepartmentOption = {
  id: string;
  name: string;
};

export type PendingBinding = {
  departmentId: string;
  roleId: string;
};
