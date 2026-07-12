export type RoleManagementSession = {
  permissions: string[];
};

export type RoleListItem = {
  id: string;
  label: string;
  description?: string;
  isSystem: boolean;
  enabled: boolean;
  permissionCount: number;
  childRoleCount: number;
  referenceCount: number;
  conditionSummary?: string;
  canEdit: boolean;
  canDelete: boolean;
  canToggleEnabled: boolean;
};

export type RolePermissionInput = {
  resource: string;
  action: string;
  attributeConditions?: {
    chan?: Array<"social" | "campus">;
    expired?: boolean[];
    self?: boolean;
  };
};

export type RoleDefinitionPayload = {
  label: string;
  description: string;
  enabled: boolean;
  permissions: RolePermissionInput[];
  childRoleIds: string[];
};

export type RoleDetail = RoleListItem & {
  permissions: RolePermissionInput[];
  childRoleIds: string[];
};

export type PermissionOptions = {
  conditionOptions?: {
    chan?: Array<"social" | "campus">;
    expired?: boolean[];
  };
  resources: Array<{
    resource: string;
    actions: Array<{
      action: string;
      supportsConditions: {
        chan: boolean;
        expired: boolean;
        self: boolean;
      };
    }>;
  }>;
};
