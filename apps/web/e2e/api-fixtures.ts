import type { Page, Route } from "@playwright/test";

export const adminSession = {
  dataScope: { allDepartments: true, channels: ["social", "campus"], departments: [] },
  defaultRoute: "/resume-parse",
  pageAccess: ["resume-parse", "resume-recommend", "resume-library", "departments-positions", "users", "roles"],
  permissions: [
    "Resume.List",
    "Resume.Get",
    "Resume.Create",
    "Resume.Update",
    "Resume.Delete",
    "Position.List",
    "Position.Get",
    "PositionResume.Create",
    "DepartmentResume.Create",
    "Notification.List",
    "Notification.Get",
    "Notification.Update",
    "Notification.Create",
    "User.List",
    "User.Get",
    "UserDepartmentRole.List",
    "UserDepartmentRole.Create",
    "UserDepartmentRole.Delete",
    "Role.List",
    "Role.Get",
    "Role.Create",
    "Role.Update",
    "Role.Delete",
    "Permission.List",
  ],
  roleBindings: [{ departmentId: "__system__", departmentName: "system", roleLabel: "超级管理员" }],
  roleLabels: ["超级管理员"],
  user: { employeeId: "A00001", id: "admin", name: "管理员" },
};

const resumeListResponse = {
  availableChannels: ["social", "campus"],
  channelCounts: { social: 1, campus: 0 },
  dataScopeSummary: "全部部门",
  items: [
    {
      id: "resume_1",
      name: "张三",
      age: 29,
      school: "浙江大学",
      yearsExp: 6,
      currentDepartment: { id: "dept_a", name: "算力训练平台部" },
      pos: "平台工程师",
      source: "导入",
      sourceBy: "李四",
      chan: "social",
      keywords: ["Go", "调度"],
      canGet: true,
      canDelete: false,
    },
  ],
  nextCursor: "",
};

const positionListResponse = {
  items: [
    {
      id: "position_1",
      name: "平台工程师",
      department: { id: "dept_a", name: "算力训练平台部" },
      chan: "social",
      level: "P6",
      status: "on",
      keywordCount: 2,
      implicitTagCount: 1,
      updatedAt: "2026-07-12T08:00:00Z",
      canGet: true,
      canUpdate: true,
      canDelete: true,
    },
  ],
};

const departmentListResponse = {
  items: [
    {
      id: "dept_a",
      name: "算力训练平台部",
      positionCount: 1,
      resumeCount: 2,
      updatedAt: "2026-07-12T08:00:00Z",
      canGet: true,
      canUpdate: true,
      canDelete: false,
    },
    {
      id: "dept_b",
      name: "智算调度部",
      positionCount: 1,
      resumeCount: 0,
      updatedAt: "2026-07-12T08:00:00Z",
      canGet: true,
      canUpdate: true,
      canDelete: false,
    },
  ],
};

const userListResponse = {
  canAssignRoles: true,
  dataScopeSummary: "全部部门",
  items: [
    {
      id: "user_a",
      employeeId: "A10001",
      name: "张敏",
      departments: [{ id: "dept_a", name: "算力训练平台部", system: false }],
      roleBindings: [
        {
          id: "udr_a",
          role: { id: "__role_hrbp__", label: "HRBP", isSystem: true, enabled: true },
          department: { id: "dept_a", name: "算力训练平台部", system: false },
          guest: false,
          createdAt: "2026-07-12T08:00:00Z",
          createdBy: "admin",
          canDelete: true,
        },
      ],
      roleSummary: "HRBP(部门:算力训练平台部)",
      guestOnly: false,
      canAssign: true,
    },
  ],
  nextCursor: "",
};

const managerBinding = {
  id: "udr_manager",
  role: { id: "__role_manager__", label: "主管", isSystem: true, enabled: true },
  department: { id: "dept_b", name: "智算调度部", system: false },
  guest: false,
  createdAt: "2026-07-12T08:01:00Z",
  createdBy: "admin",
  canDelete: false,
};

function userListFixture({
  hrbpBindingPresent,
  managerBindingPresent,
}: {
  hrbpBindingPresent: boolean;
  managerBindingPresent: boolean;
}) {
  const user = userListResponse.items[0];
  const roleBindings = [
    ...(hrbpBindingPresent ? user.roleBindings : []),
    ...(managerBindingPresent ? [managerBinding] : []),
  ];
  return {
    ...userListResponse,
    items: [
      {
        ...user,
        roleBindings,
        roleSummary: roleBindings
          .map((binding) => `${binding.role.label}(部门:${binding.department.name})`)
          .join("、"),
      },
    ],
  };
}

const assignableRolesResponse = {
  items: [
    {
      id: "__role_hrbp__",
      label: "HRBP",
      description: "部门 HRBP",
      isSystem: true,
      supportsSystemDepartment: false,
      attributeConditionSummary: "",
    },
    {
      id: "__role_manager__",
      label: "主管",
      description: "业务主管",
      isSystem: true,
      supportsSystemDepartment: false,
      attributeConditionSummary: "",
    },
  ],
};

const roleListResponse = {
  canCreate: true,
  total: 1,
  items: [
    {
      id: "__role_hrbp__",
      label: "HRBP",
      description: "部门 HRBP",
      isSystem: true,
      enabled: true,
      permissionCount: 3,
      childRoleCount: 0,
      referenceCount: 2,
      conditionSummary: "全部渠道",
      canEdit: true,
      canDelete: false,
      canToggleEnabled: true,
    },
  ],
};

const rolePermissionOptionsResponse = {
  conditionOptions: { chan: ["social", "campus"], expired: [false, true] },
  resources: [
    {
      resource: "Resume",
      actions: [{ action: "List", supportsConditions: { chan: true, expired: true, self: false } }],
    },
  ],
};

const notification = {
  id: "notification_1",
  resumeId: "resume_copy",
  candidateName: "张三",
  department: { id: "dept_b", name: "智算调度部" },
  recommender: { id: "admin", name: "管理员" },
  chan: "social",
  createdAt: "2026-07-12T08:00:00Z",
  read: false,
  canOpenResumeLibrary: true,
};

export async function installApiMocks(page: Page) {
  let unreadCount = 2;
  let notificationUnread = true;
  let customReviewerCreated = false;
  let hrbpBindingPresent = true;
  let managerBindingPresent = false;

  await page.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (url.pathname === "/me" && method === "GET") {
      return json(route, adminSession);
    }
    if (url.pathname === "/notifications/summary" && method === "GET") {
      return json(route, { unreadCount });
    }
    if (url.pathname === "/resumes" && method === "GET") {
      return json(route, resumeListResponse);
    }
    if (url.pathname === "/positions" && method === "GET") {
      return json(route, positionListResponse);
    }
    if (url.pathname === "/users" && method === "GET") {
      return json(route, userListFixture({ hrbpBindingPresent, managerBindingPresent }));
    }
    if (url.pathname === "/departments" && method === "GET") {
      return json(route, departmentListResponse);
    }
    if (url.pathname === "/roles/assignable" && method === "GET") {
      return json(route, assignableRolesResponse);
    }
    if (url.pathname === "/users/user_a/role-bindings" && method === "POST") {
      if (
        !(await validateJsonBody(route, {
          bindings: [{ departmentId: "dept_b", roleId: "__role_manager__" }],
        }))
      ) {
        return;
      }
      managerBindingPresent = true;
      return json(route, {
        message: "已为 张敏 分配 1 个角色绑定",
        created: [managerBinding],
        user: { id: "user_a", employeeId: "A10001", name: "张敏" },
      });
    }
    if (url.pathname === "/users/user_a/role-bindings/udr_a" && method === "DELETE") {
      hrbpBindingPresent = false;
      return json(route, {
        deletedBindingId: "udr_a",
        userId: "user_a",
        message: "已解除 HRBP(部门:算力训练平台部)",
      });
    }
    if (url.pathname === "/roles" && method === "GET") {
      return json(route, {
        ...roleListResponse,
        items: customReviewerCreated
          ? [
              ...roleListResponse.items,
              {
                id: "role_custom_reviewer",
                label: "高级评审者",
                description: "查看社招简历",
                isSystem: false,
                enabled: true,
                permissionCount: 1,
                childRoleCount: 0,
                referenceCount: 0,
                conditionSummary: "社招",
                canEdit: true,
                canDelete: true,
                canToggleEnabled: true,
              },
            ]
          : roleListResponse.items,
        total: customReviewerCreated ? 2 : 1,
      });
    }
    if (url.pathname === "/roles/permission-options" && method === "GET") {
      return json(route, rolePermissionOptionsResponse);
    }
    if (url.pathname === "/roles" && method === "POST") {
      const body = await readJsonBody(route);
      if (!body) {
        return;
      }
      if (body.label === "循环角色") {
        if (
          !(await validateJsonBody(route, {
            label: "循环角色",
            description: "",
            enabled: true,
            permissions: [],
            childRoleIds: ["__role_hrbp__"],
          }))
        ) {
          return;
        }
        return json(route, { code: "IAM_ROLE_RELATION_CYCLE", message: "角色包含关系不能形成循环。" }, 400);
      }
      if (
        !(await validateJsonBody(route, {
          label: "高级评审者",
          description: "查看社招简历",
          enabled: true,
          permissions: [
            {
              resource: "Resume",
              action: "List",
              attributeConditions: { chan: ["social"] },
            },
          ],
          childRoleIds: [],
        }))
      ) {
        return;
      }
      customReviewerCreated = true;
      return json(route, { id: "role_custom_reviewer", label: "高级评审者" });
    }
    if (url.pathname === "/matching/parse" && method === "POST") {
      if (!(await validateJsonBody(route, { resumeId: "resume_1", positionId: "position_1" }))) {
        return;
      }
      return json(route, {
        id: "position_resume_1",
        resume: {
          id: "resume_1",
          name: "张三",
          chan: "social",
          currentDepartment: { id: "dept_a", name: "算力训练平台部" },
          keywords: ["Go", "调度"],
          traits: ["稳定"],
          expBase: 82,
        },
        position: {
          id: "position_1",
          name: "平台工程师",
          department: { id: "dept_a", name: "算力训练平台部" },
          chan: "social",
          level: "P6",
          status: "on",
          keywords: ["Go", "Kubernetes"],
          implicitTags: [{ name: "稳定", w: 40 }],
        },
        score: { total: 76, skill: 50, experience: 82, implicit: 100, judgement: "建议进入面试" },
        evidence: {
          keywords: [
            { name: "Go", matched: true },
            { name: "Kubernetes", matched: false },
          ],
          implicitTags: [{ name: "稳定", w: 40, matched: true }],
          analysis: "技能命中 1/2；隐性要求命中 1/1；建议进入面试。",
        },
        createdAt: "2026-07-12T08:00:00Z",
      });
    }
    if (url.pathname === "/matching/interview-questions" && method === "POST") {
      if (!(await validateJsonBody(route, { resumeId: "resume_1", positionId: "position_1", matchScore: 76 }))) {
        return;
      }
      return json(route, {
        groups: [
          {
            type: "professional",
            label: "专业面试",
            questions: [{ order: 1, question: "请介绍 Go 项目", why: "验证经验", difficulty: "核心" }],
          },
          {
            type: "manager",
            label: "主管面试",
            questions: [
              {
                order: 1,
                question: "为什么选择算力训练平台部，以及你期待如何协作？",
                why: "确认动机",
                difficulty: "动机",
              },
            ],
          },
          {
            type: "qualification",
            label: "资格面试",
            questions: [{ order: 1, question: "请确认到岗时间", why: "确认流程", difficulty: "流程" }],
          },
        ],
      });
    }
    if (url.pathname === "/recommendations/route" && method === "POST") {
      if (!(await validateJsonBody(route, { resumeId: "resume_1" }))) {
        return;
      }
      return json(route, {
        resume: {
          id: "resume_1",
          name: "张三",
          chan: "social",
          pos: "平台工程师",
          currentDepartment: { id: "dept_a", name: "算力训练平台部" },
          keywords: ["Go"],
        },
        routes: [
          {
            department: { id: "dept_b", name: "智算调度部" },
            position: { id: "position_b", name: "调度工程师", chan: "social", level: "P6" },
            score: { total: 86, skill: 100, experience: 82, implicit: 80, judgement: "强烈推荐" },
            contacts: { hrbps: ["李四"], managers: ["王五"], trainees: ["赵六"] },
            best: true,
          },
        ],
        createdAt: "2026-07-12T08:00:00Z",
      });
    }
    if (url.pathname === "/recommendations/send" && method === "POST") {
      if (!(await validateJsonBody(route, { resumeId: "resume_1", departmentId: "dept_b", positionId: "position_b" }))) {
        return;
      }
      notificationUnread = true;
      unreadCount = Math.max(unreadCount, 2);
      return json(route, {
        resumeId: "resume_copy",
        sourceResumeId: "resume_1",
        department: { id: "dept_b", name: "智算调度部" },
        position: { id: "position_b", name: "调度工程师" },
        candidateName: "张三",
        reusedExistingCopy: false,
        notifiedCount: 4,
        selfNotificationRead: true,
        message: "已推荐到「智算调度部」· 已通知 4 位相关人员",
      });
    }
    if (url.pathname === "/notifications" && method === "GET") {
      return json(route, { items: notificationUnread ? [notification] : [], unreadCount, nextCursor: "" });
    }
    if (url.pathname === "/notifications/notification_1/read" && method === "POST") {
      notificationUnread = false;
      unreadCount = 1;
      return json(route, { notification: { ...notification, read: true }, unreadCount });
    }
    if (url.pathname === "/notifications/read-all" && method === "POST") {
      notificationUnread = false;
      unreadCount = 0;
      return json(route, { updatedCount: 1, unreadCount });
    }
    return route.fallback();
  });
}

async function readJsonBody(route: Route): Promise<Record<string, unknown> | undefined> {
  try {
    const body = route.request().postDataJSON();
    if (!body || typeof body !== "object" || Array.isArray(body)) {
      await json(route, { code: "E2E_FIXTURE_INVALID_JSON", message: "Expected a JSON object request body." }, 400);
      return undefined;
    }
    return body as Record<string, unknown>;
  } catch {
    await json(route, { code: "E2E_FIXTURE_INVALID_JSON", message: "Expected a JSON request body." }, 400);
    return undefined;
  }
}

async function validateJsonBody(route: Route, expected: unknown) {
  let actual: unknown;
  try {
    actual = route.request().postDataJSON();
  } catch {
    await json(route, { code: "E2E_FIXTURE_INVALID_JSON", message: "Expected a JSON request body.", expected }, 400);
    return false;
  }

  if (!isDeepEqual(actual, expected)) {
    await json(
      route,
      {
        actual,
        code: "E2E_FIXTURE_UNEXPECTED_BODY",
        expected,
        message: "Request body did not match the e2e fixture expectation.",
      },
      400,
    );
    return false;
  }

  return true;
}

function isDeepEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) {
    return true;
  }
  if (!left || !right || typeof left !== "object" || typeof right !== "object") {
    return false;
  }
  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) {
      return false;
    }
    return left.every((item, index) => isDeepEqual(item, right[index]));
  }

  const leftRecord = left as Record<string, unknown>;
  const rightRecord = right as Record<string, unknown>;
  const leftEntries = Object.entries(leftRecord);
  const rightEntries = Object.entries(rightRecord);
  if (leftEntries.length !== rightEntries.length) {
    return false;
  }

  return leftEntries.every(([key, value]) => Object.prototype.hasOwnProperty.call(rightRecord, key) && isDeepEqual(value, rightRecord[key]));
}

export async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    body: JSON.stringify(body),
    contentType: "application/json",
    status,
  });
}
