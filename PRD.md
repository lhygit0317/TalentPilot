# 算力事业部 · 招聘智能助手需求文档

> 文档类型:产品需求文档(PRD)
> 文档范围:登录认证、权限模型、简历解析、智能分流、简历库、部门岗位管理、用户角色管理、通知提醒
> 验收口径:本文中的 Story 与 AC 为功能验收依据;所有权限相关行为以后端鉴权结果为准

---

## 0. 文档说明

### 0.1 目的

本文定义招聘智能助手的业务目标、权限模型、数据模型、功能范围、验收条件与非功能要求,作为产品、研发、测试和业务验收的统一依据。

### 0.2 受众

- **产品/PM**:确认范围、优先级、边界和验收口径
- **开发/测试**:拆分任务、设计接口、实现功能并编写测试用例
- **业务/HR**:走查业务流程,确认角色权限和操作边界

### 0.3 术语表

| 术语 | 含义 |
|------|------|
| W3 | 公司内部统一身份认证系统 |
| JD | Job Description,岗位职责描述 |
| **HRD** | HRBP 的领导(对所辖 HRBP 有管理权) |
| **HRBP** | 业务伙伴;**一人可负责多个部门** |
| **主管** | 部门的领导;**一人可领导多个部门** |
| 锻炼干部 | 业务方承担招聘协同角色的同事;在 UserDepartmentRole 中以"锻炼干部"角色绑定到部门 |
| 在架/下架 | 岗位是否参与简历解析与分流匹配 |
| 隐性标签 | HRBP 在 JD 之外补充的软性要求(如"抗压稳定"),带权重 |
| **资源 (Resource)** | 系统中的实体对象:User / Department / Position / Resume / Role / Notification / Permission / UserDepartmentRole / DepartmentPosition / DepartmentResume / PositionResume / RoleRelation |
| **操作 (Action)** | 对资源的 CRUD 五操作:**Create / Get / List / Update / Delete** |
| **角色 (Role)** | 权限标签;不区分原子角色和复合角色 |
| **权限 (Permission)** | 角色对 (资源, 操作) 的允许项;一条记录 = 一个 (roleId, resource, action),可选携带 attributeConditions |
| **角色关联 (RoleRelation)** | 角色 A "包含"角色 B 的权限;**有向、可递归** |
| **部门关系 (4 个显式关系)** | 用户-部门-角色 / 部门-岗位 / 部门-简历 / 岗位-简历(详见 2.2) |
| **UserDepartmentRole** | 三元关系 (user, department, role);定义"某用户在某部门拥有某角色" |
| **DepartmentPosition** | 二元关系;定义"某岗位归属某部门" |
| **DepartmentResume** | 二元关系;定义"某简历当前归属某部门" |
| **PositionResume** | 二元关系;定义"某简历与某岗位的关联历史"(解析/推荐/手动);其中 `kind` 是该关系记录的类型字段,不是简历属性,也不是 Permission 的属性条件 |
| **属性条件 (AttributeCondition)** | Permission 上的可选数据过滤条件;如 `resource=Resume` 时按 `attributeConditions.chan=social` 控制社招简历权限 |
| **游客 (Guest)** | W3 首次登录但尚未被分配业务角色的用户,默认最低权限 |

---

## 1. 权限模型

> **设计原则**:权限 = 资源 + CRUD 操作;角色通过 RoleRelation 实现复用与继承;部门、岗位、简历、角色之间的业务关系通过显式关系实体表达。

### 1.1 资源清单(Resources)

| 资源 | 说明 |
|------|------|
| User | 系统用户(W3 登录写入) |
| Department | 部门基本信息 |
| Position | 岗位(JD + 关键词 + 隐性标签 + 状态) |
| Resume | 简历 |
| Role | 角色定义 |
| Notification | 通知消息 |
| Permission | 角色对资源的操作许可(也可视为资源) |
| UserDepartmentRole | 用户-部门-角色三元关系 |
| DepartmentPosition | 部门-岗位二元关系 |
| DepartmentResume | 部门-简历二元关系 |
| PositionResume | 岗位-简历二元关系 |
| RoleRelation | 角色之间的包含关系 |

### 1.2 操作清单 — CRUD 五操作

系统统一使用 5 种资源操作,**所有用户行为都必须落到这 5 种操作之一**:

| 操作 | 适用场景 | 说明 |
|------|----------|------|
| **Create** | 新建/导入/分配/推荐 | 创建新记录(批量视为多条 Create) |
| **Get** | 查看单条详情 | 读取单条记录(如简历详情、JD 详情、通知详情) |
| **List** | 查看列表 / 搜索 / 导出 | 读取多条记录;**导出 = List + 格式转换** |
| **Update** | 编辑 / 上下架 / 改状态 / 解析(写入解析结果) | 更新已有记录;**上下架 = Update Position.status** |
| **Delete** | 删除 / 解除 | 移除记录 |

**常见用户行为的 CRUD 映射**(用于 Story AC 引用):

| 用户行为 | 落到的 CRUD 操作 |
|----------|------------------|
| 导入简历 | Create Resume + Create DepartmentResume |
| 批量导入 | N × (Create Resume + Create DepartmentResume) |
| 解析简历 | Create PositionResume(kind=parsed) + List 候选人关键词匹配详情 |
| 推荐简历 | List 查重 + Create/Update Resume(副本) + Create DepartmentResume(新副本时) + Create/Update PositionResume(kind=recommended) + Create N 条 Notification |
| 上下架岗位 | Update Position(status) |
| 编辑 JD | Update Position |
| 维护隐性标签 | Update Position.implicit |
| 删除简历 | Delete Resume(级联 Delete DepartmentResume / PositionResume) |
| 导出面试题 | List PositionResume + 格式转换 |
| 角色分配 | Create UserDepartmentRole |
| 角色解绑 | Delete UserDepartmentRole |
| W3 登录用户入库 | Create User(若新) / Update User.name/employeeId(若已存在) |
| 标记通知已读 | Update Notification(read=true) |

### 1.3 角色(Role)定义

角色是权限标签,不区分原子角色和复合角色。

**角色的"内容"完全由两个关系实体表达**:
- `Permission(roleId, resource, action, attributeConditions?)`:角色允许对哪些资源做哪些 CRUD 操作,并可附带一组可选 attribute conditions 做数据权限过滤
- `RoleRelation(parentRoleId, childRoleId)`:角色 A 的权限 = 自身 Permission ∪ 角色 B 的所有权限(递归)

**角色元数据字段**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 角色 ID |
| label | string | 角色显示名(全局唯一) |
| description | string | 角色描述(可选) |
| **isSystem** | bool | **true = 系统预置,不可删除;false = 超级管理员可删** |
| **enabled** | bool | **true = 在角色下拉中可见;false = 仅历史绑定可见** |
| createdAt, createdBy, updatedAt | timestamp / userId | 审计字段 |

**系统初始化预置角色**(均为 isSystem=true,enabled=true):

| label | 默认 Permission 范围 | 默认数据条件 | 说明 |
|-------|----------------------|-----------|------|
| 游客 (guest) | Department.List、User.Get(self) | 无 | W3 首次登录默认授予 |
| HRBP | User(List/Get public)+ Department(List/Get)+ UserDepartmentRole(List)+ DepartmentPosition(List)+ Resume(List/Get/Create/Update/Delete)+ Position(List/Get)+ DepartmentResume(Create)+ PositionResume(Create)+ Notification(List/Get/Create/Update) | 无 | 通过 UserDepartmentRole 绑定到部门;可查看部门/岗位数据,满足解析、导入、推荐和推荐结果联系人展示所需 CRUD |
| HRD | HRBP 全部 + 主管全部 + 锻炼干部全部 + UserDepartmentRole(Create/Delete)+ DepartmentResume(Create) | 无 | 通过 UserDepartmentRole 绑定到部门(可为特殊 system 部门以覆盖全部);默认不具备部门/岗位写权限和角色管理权限 |
| 主管 | User(List/Get public)+ Department(List/Get)+ UserDepartmentRole(List)+ DepartmentPosition(List)+ Resume(List/Get/Create)+ Position(List/Get)+ DepartmentResume(Create)+ PositionResume(Create)+ Notification(Create) | 无 | 通过 UserDepartmentRole 绑定到部门;可参与解析/推荐但默认不可删改简历和岗位 |
| 锻炼干部 | User(List/Get public)+ Department(List/Get)+ UserDepartmentRole(List)+ DepartmentPosition(List)+ Resume(List/Get)+ Position(List/Get)+ PositionResume(Create) | 无 | 通过 UserDepartmentRole 绑定到部门;用于业务协同与分流展示 |
| 社招负责人 | User(List/Get public)+ Department(List/Get)+ UserDepartmentRole(List)+ DepartmentPosition(List)+ Resume(List/Get/Create/Update/Delete)+ Position(List/Get)+ DepartmentResume(Create)+ PositionResume(Create)+ Notification(List/Get/Create/Update) | Resume.attributeConditions: `chan=social` | 跨部门,通过 Permission.attributeConditions 自动过滤社招简历 |
| 校招负责人 | 同社招负责人 | Resume.attributeConditions: `chan=campus` | 跨部门,通过 Permission.attributeConditions 自动过滤校招简历 |
| 超级管理员 | 所有资源的全部 CRUD | 无 | 系统级全部权限;唯一默认拥有部门管理和岗位管理写权限的角色 |

> 注:上述"默认 Permission 范围"为业务上对该角色的能力预期描述;实现时超级管理员可在 E8 角色管理中按需调整。**预置角色的 RoleRelation 关系见 1.5**。

### 1.4 角色与资源的关联(Permission)

**Permission 表**存储"角色允许对哪些资源做哪些 CRUD 操作":

```
Permission {
  id: string
  roleId: string      // 引用 Role
  resource: enum      // User / Department / Position / Resume / Role / Notification / Permission / UserDepartmentRole / DepartmentPosition / DepartmentResume / PositionResume / RoleRelation
  action: enum        // Create / Get / List / Update / Delete
  attributeConditions?: json   // 可选的数据权限条件,如 { chan: ["social"], expired: [false] }
}
```

**权限表达分两层**:
- 大类权限由 `(resource, action)` 控制,如 `Resume.List`
- 数据权限由 `attributeConditions` 控制资源属性,如 `Resume.List + attributeConditions.chan=["social"]`
- 渠道、是否过期等数据权限以 `Resume.attributes` 为准;`Role` 不再持有渠道字段

**Permission 矩阵边界说明**:
- `(resource, action)` 是系统内部权限表达模型,不代表角色配置页要把 12 个资源 × 5 个 CRUD 的 60 个组合全部开放给业务随意勾选
- 角色配置页只展示产品定义的 Permission 白名单;无业务意义或风险过高的组合不展示、也不允许通过接口保存
- 例:部门 / 岗位写权限只对白名单中的超级管理员开放;普通业务角色只看到 Department.List/Get、Position.List/Get 等只读组合
- 数据范围收窄不通过新增资源类型解决,而通过 `attributeConditions` 表达,如 Resume.List + `chan=social`、`expired=false`
- 若后续业务需要新增某个 Permission 组合,需先由产品 / 研发确认并加入白名单,再允许在角色管理中配置

**示例**(超级管理员):

```
Permission(roleId=超级管理员, resource=Resume, action=List)
Permission(roleId=超级管理员, resource=Resume, action=Get)
Permission(roleId=超级管理员, resource=Resume, action=Create)
Permission(roleId=超级管理员, resource=Resume, action=Update)
Permission(roleId=超级管理员, resource=Resume, action=Delete)
Permission(roleId=超级管理员, resource=Position, action=List)
...
(超级管理员拥有所有资源的所有 CRUD)
```

**Permission 检查逻辑**(运行时):

```
user_can(session, action, resource, target?):
  1. 若无有效登录会话 → 拒绝
     - W3 侧非在职 / 账号不存在在登录阶段已失败,不会进入业务鉴权
  2. 获取 session.user 的所有 UserDepartmentRole 记录,逐条作为独立授权来源
  3. 对每条绑定 b(user, dept, role),通过 RoleRelation 展开 role:
     - 展开时跳过 enabled=false 的子角色
     - RoleRelation 必须无直接或间接循环
  4. 对每个可达角色 r,查找 Permission(r, resource, action)
  5. 命中 Permission 后生成该绑定的访问谓词 P(b,r,p):
     - 部门谓词:若 b.departmentId=system → 全部门;否则按资源映射到 b.departmentId
       * Resume:通过 DepartmentResume.departmentId 反查
       * Position:通过 DepartmentPosition.departmentId 反查
       * Department / DepartmentPosition / DepartmentResume / UserDepartmentRole:直接按 departmentId 判断
     - 属性谓词:若 Permission.attributeConditions 非空 → 目标资源属性必须满足 attributeConditions
       * 例:resource=Resume 且 attributeConditions.chan=["social"] → 仅允许 Resume.attributes.chan=social
       * 未配置 attributeConditions → 不按属性额外过滤
  6. 用户最终允许范围 = 所有命中 Permission 的 P(b,r,p) 取并集(OR)
  7. target 落入任一允许范围 → 允许;否则拒绝
```

### 1.5 角色与角色的关联(RoleRelation)

**RoleRelation 表**存储"角色 A 包含角色 B 的所有权限":

```
RoleRelation {
  id: string
  parentRoleId: string   // 父角色(A)
  childRoleId: string    // 子角色(B),父角色包含其所有权限
}
```

**特性**:
- **有向**:`(A → B)` 表示 A 包含 B,**B 不自动包含 A**
- **可递归**:`A → B → C` 等价于 `A → B` + `A → C` + `B → C`(系统自动展开)
- **可一对多**:A 可包含 B、C、D 多个子角色

**系统预置 RoleRelation**(实现"包含"语义):

```
RoleRelation(HRD, HRBP)            // HRD 包含 HRBP 的所有权限
RoleRelation(HRD, 主管)            // HRD 包含主管
RoleRelation(HRD, 锻炼干部)        // HRD 包含锻炼干部
RoleRelation(超级管理员, HRD)          // 超级管理员包含 HRD
RoleRelation(超级管理员, 社招负责人)   // 超级管理员包含社招负责人
RoleRelation(超级管理员, 校招负责人)
```

**用户自建 RoleRelation**(Story 8.3):
- 超级管理员可创建任意 RoleRelation,但**禁止任何直接或间接循环引用**(如 A→B→A 或 A→B→C→A)
- 删除子角色时,父角色对其的 RoleRelation 自动解除
- 编辑/删除 RoleRelation 后,所有相关 UserDepartmentRole 立即按新关系生效

### 1.6 角色保护规则

| 角色类别 | 是否可删除 | 是否可编辑 Permission/RoleRelation | 是否可编辑 label/description/enabled |
|----------|:----------:|:----------------------------------:|:--------------------------------------------:|
| **系统预置角色**(游客、HRBP、HRD、主管、社招/校招负责人、超级管理员) | ✗ | ✓ | ✓ |
| **超级管理员创建的自定义角色** | ✓(无引用时) | ✓ | ✓ |

补充规则:
- 角色被任何 UserDepartmentRole 引用时,删除前必须先解除所有引用
- 编辑 Permission / RoleRelation 后,已存在的 UserDepartmentRole **立即按新权限生效**(无需重新绑定)
- 系统预置角色不可删除,但可被"软禁用"(设置 `enabled=false`,下拉中不展示,保留定义供回滚)
- 自定义角色仅在引用计数 = 0 时可删除;历史引用通过审计日志保留当时的 roleId / roleLabel 快照,不保留指向已删除 Role 的 UserDepartmentRole
- 删除 RoleRelation:仅解除两角色的包含关系,不影响两角色各自的定义

### 1.7 角色聚合规则

- 用户拥有的角色 = 所有 UserDepartmentRole(userId, *, *) 展开得到的角色集合
- 用户最终权限 = 通过 RoleRelation 递归展开后所有角色的 Permission **并集**
- 部门作用域 = 用户的 UserDepartmentRole 中,`departmentId` 的并集(若包含特殊 system 部门则视为全部门)
- 渠道属性条件 = 命中的 Resume Permission.attributeConditions 中 `chan` 的并集(可多个,如同时是社招+校招负责人)
- **典型场景**:
  - 张敏 = UserDepartmentRole(张敏, 算力训练平台部, HRBP) + UserDepartmentRole(张敏, 智算调度部, HRBP) + UserDepartmentRole(张敏, 硬件加速部, 主管)
    → 部门作用域 = {算力训练平台部, 智算调度部, 硬件加速部},渠道属性条件 = ∅(可看所有渠道)
  - 李建国 = UserDepartmentRole(李建国, system, HRD)
    → 部门作用域 = 全部门(因 HRD 无 Resume.attributes.chan 条件,且绑定到 system 部门时按 HRD 默认覆盖全部门规则展开),渠道属性条件 = ∅
  - 孙磊 = UserDepartmentRole(孙磊, system, 社招负责人)
    → 部门作用域 = 全部门,渠道属性条件 = {social}(由社招负责人角色的 Resume Permission.attributeConditions 得出)
  - 赵晨 = UserDepartmentRole(赵晨, 算力训练平台部, HRBP) + UserDepartmentRole(赵晨, system, 社招负责人)
    → Resume 允许范围 = 算力训练平台部全部渠道 ∪ 全部门社招渠道
- HRD 默认行为:当 UserDepartmentRole(某用户, 任意具体部门, HRD) → 仅在该部门有 HRD 权限;当 (某用户, system 部门, HRD) → 覆盖全部门。**超级管理员或授权管理角色通过绑定到 system 部门实现"HRD 全部门权限"**。

### 1.8 属性条件(AttributeCondition)

**定义**:某些权限天然只允许访问某类资源属性,如社招负责人只能访问 `Resume.attributes.chan=social` 的简历。此类条件不放在 Role 上,而放在 Permission.attributeConditions 上:

- `Permission(roleId=社招负责人, resource=Resume, action=List, attributeConditions={ chan: ["social"] })`
- `Permission(roleId=校招负责人, resource=Resume, action=List, attributeConditions={ chan: ["campus"] })`
- 属性条件只作用于对应 Permission 的 resource/action,不会自动影响其他资源
- 一个用户可同时命中多个不同 AttributeCondition,最终查询条件取并集,如 `chan IN (social, campus)`
- 超级管理员的 Resume Permission 不配置 `attributeConditions`,可看所有渠道和过期状态

## 2. 数据模型概要

### 2.1 核心实体

```
User       { id, employeeId, name, createdAt }
Department { id, name }
Position   { id, name, chan, level, status, duties[], must[], keywords[], implicit[] }
Resume     { id, name, age?, school?, yearsExp?, pos, source, sourceBy, createdAt, attributes{chan, expired}, keywords[], traits[], expBase, profile{} }
Role       { id, label, description, isSystem, enabled, createdAt, createdBy, updatedAt }
Permission { id, roleId, resource, action, attributeConditions? }
RoleRelation { id, parentRoleId, childRoleId }
Notification { id, to, resumeId, departmentId, positionId?, name, by, chan, time, read }
```

### 2.2 关系实体(4 个显式关系)

系统通过显式关系实体表达用户、部门、岗位、简历之间的关联,避免把业务关系嵌入核心实体字段。

```
// 三元关系:某用户在某部门拥有某角色
UserDepartmentRole {
  id, userId, departmentId, roleId,
  createdAt, createdBy
}

// 二元关系:某岗位归属某部门
DepartmentPosition {
  id, departmentId, positionId,
  createdAt
}

// 二元关系:某简历当前归属某部门
DepartmentResume {
  id, departmentId, resumeId,
  assignedAt, by
}

// 二元关系:某简历与某岗位的关联历史(解析/推荐/手动)
PositionResume {
  id, positionId, resumeId,
  kind,           // parsed | recommended | manual
  matchScore?,    // 仅 kind=parsed 时存在
  createdAt, by?
}
```

### 2.3 关键字段约束

- **User**:
  - `id` = W3 用户 id(登录认证成功后写入,作为本系统 User 主键)
  - `employeeId` = W3 工号(登录认证成功后写入,唯一)
  - `name` = W3 姓名(登录认证成功后写入或刷新)
  - 不设置本系统账号启停状态;是否允许登录由 W3 账号存在且在职决定
  - W3 非在职 / 账号不存在的用户无法通过 W3 登录,系统不创建"非在职"用户状态
  - 不再持有 `dept` 字段(从 UserDepartmentRole 推导)
- **Department**:
  - 有特殊 `system` 部门(id=`__system__`),用于"系统级"角色绑定(如超级管理员 / 社招负责人)
- **Position**:
  - `status` ∈ {on, off}
  - `chan` ∈ {social, campus}(展示层映射为社招 / 校招)
- **Resume**:
  - `name` = 候选人姓名
  - `age` = 年龄(可选,无法解析时为空)
  - `school` = 毕业学校(可选)
  - `yearsExp` = 工作年限(可选)
  - `attributes.chan` ∈ {social, campus}(展示层映射为社招 / 校招)
  - `attributes.expired` ∈ {true, false};用于标记简历是否过期,默认 false
  - `createdAt` = 简历创建入库时间
  - `sourceBy` = 来源人;当 `source=导入` 时为导入人,当 `source=推荐` 时为推荐人
  - `profile` = 结构化简历信息(JSON),建议包含:basic、education[]、workExperience[]、projects[]、skills[]、certificates[]、rawTextRef?
  - 不持有 `curDept` / `owner` 字段;当前部门通过 DepartmentResume 反查,HRBP 通过 UserDepartmentRole 反查
  - `source` ∈ {导入, 推荐}
- **Role**:
  - `isSystem=true` 的角色 label **不可修改**(避免破坏 1.5 中的 RoleRelation)
  - `enabled=false` 时,从角色下拉中消失,但已存在的 UserDepartmentRole 仍生效
- **UserDepartmentRole**:
  - 同一 (userId, departmentId, roleId) **不可重复创建**
  - 删除用户最后一条 UserDepartmentRole 时,系统自动追加一条游客绑定(Story 6.4 边界)
- **DepartmentResume**:
  - 系统按"一份简历当前只归属一个部门"实现:同一 `resumeId` 仅允许一条有效 DepartmentResume
  - 推荐到其他部门时通过创建简历副本实现流转,不把同一个 resumeId 直接绑定到多个部门
- **Notification**:
  - `resumeId` / `chan` 用于点击通知后定位简历库渠道和候选人;`chan` 写入 `Resume.attributes.chan` 的快照
  - 若关联简历已删除,通知仍保留,但点击后提示"关联简历已删除"

### 2.4 实体关系图(完整)

```
   User ──┬── UserDepartmentRole ── Department ── DepartmentPosition ── Position
          │           │                │                                  │
          │           └─ Role          └─ DepartmentResume ── Resume ───┘
          │                              │                                  │
          └─ Notification                └─ DepartmentResume.Department     └─ PositionResume
                                                              ↑
                                                         简历归属部门

   Role ── Permission (CRUD allowlist)
   Role ── RoleRelation ── Role  (递归包含)
```

---

## 3. Epic 总览

| Epic | 名称 | Story 数 | 关键依赖 |
|------|------|----------|----------|
| E1 | 登录与身份认证 | 3 | — |
| E2 | 简历解析 | 6 | E1, E5(在架岗位) |
| E3 | 简历推荐/智能分流 | 3 | E1, E4(简历库), E5(在架岗位) |
| E4 | 简历库 | 8 | E1, E5(部门归属) |
| E5 | 部门与岗位管理 | 6 | E1, E6(用户列表) |
| E6 | 用户与角色管理 | 4 | E1, E8(角色列表) |
| E7 | 通知与提醒 | 4 | E1, E3(推荐触发) |
| E8 | 角色管理(自定义) | 5 | E1, E6(权限校验) |

合计 **39 个 Story**。

---

## 4. Epic 详写

### Epic 1: 登录与身份认证

#### Story 1.1 — 通过 W3 统一认证登录

- **作为**:任何公司内部用户
- **我想**:在登录页输入公司域账号和密码并登录
- **以便**:用公司统一身份进入招聘助手
- **触发场景**:登录页,用户未登录
- **AC**:
  - Given 我在登录页,When 输入公司域账号和密码并提交,Then 系统调用 W3 认证
  - And TalentPilot 仅在后端内存中转发账号密码给 W3 API,不得保存密码或把密码写入日志 / 审计 / 错误详情
  - And W3 仅允许在职员工认证成功;若 W3 账号不存在或非在职,认证失败,本系统不创建用户
  - And 认证成功后,W3 回传 `id` + `name` + `employeeId`(工号)
  - And 系统按 W3 `id` 查找本系统用户:
    - 若用户已存在:**Update User(name, employeeId)** 后加载其 UserDepartmentRole 集合,登录成功,跳默认主页
    - 若本系统用户不存在:**Create User(id+name+employeeId),Create UserDepartmentRole(userId=id, system, 游客角色),登录成功**
  - And 登录后统一跳转到默认主页"简历解析"(游客也进入该页,见 Story 1.2)
  - And 顶部 toast 提示"已通过 W3 登录 · {角色 label 列表}"
- **边界用例**:
  - W3 认证失败:停留登录页,提示失败原因
  - W3 接口超时:重试 1 次,失败则提示
- **备注**:生产环境接入真实 W3

#### Story 1.2 — 首次登录与游客页面权限

- **作为**:W3 首次登录的本系统新用户(尚无 UserDepartmentRole 业务绑定)
- **我想**:登录后默认进入"简历解析"页,并只看到游客可访问的业务页面
- **以便**:在有限页面范围内使用系统,同时等待管理员分配业务角色
- **触发场景**:Story 1.1 中 W3 回传的 `id` 在本系统中无记录时
- **AC**:
  - Given W3 登录成功且本系统无该 W3 `id` 记录,Then 系统自动:
    - **Create User**(id + name + employeeId)
    - **Create UserDepartmentRole**(userId, system 部门, 游客角色)
  - And 登录后跳转到默认主页"简历解析"
  - And 游客可见页面仅包含"简历解析"和"简历推荐",不展示游客空状态
  - And 其他业务页面入口隐藏或不可访问
  - And 前端页面可见性不作为安全边界;资源操作仍以后端权限校验结果为准
- **边界用例**:
  - 超级管理员或授权管理角色后续给该用户 Create UserDepartmentRole(具体部门, 业务角色):用户重新登录后,默认主页切换,业务菜单可用
  - 游客尝试直接访问无权限业务 URL(改 URL):后端鉴权拒绝,前端引导回默认主页"简历解析"

#### Story 1.3 — 切换账号与退出登录

- **作为**:已登录用户
- **我想**:点右上头像,选择"退出登录 / 切换账号"
- **以便**:退出当前会话或重新通过 W3 认证
- **触发场景**:任意已登录页面,顶部右上角用户菜单
- **AC**:
  - Given 我已登录,When 点击右上角头像按钮,Then 弹出下拉菜单
  - And 菜单顶部展示:姓名、工号、**当前角色集合(含部门)**,格式如 `HRBP(部门:算力训练平台部、智算调度部) | 主管(部门:硬件加速部)`
  - When 点击"退出登录 / 切换账号",Then 退出到登录页,会话清空
- **边界用例**:
  - 重新登录后:解析/推荐/简历库的 UI 状态恢复到默认(社招/未选简历)
  - 多角色用户:下拉中按 UserDepartmentRole 列表逐条展示

---

### Epic 2: 简历解析

> 入口:左侧导航"简历解析"。依赖:E5 提供在架岗位,E4 提供简历数据。
> **访问条件**:用户拥有 `Resume.List` + `Resume.Get` + `Position.List` + `PositionResume.Create` 权限(组合起来等价于"简历解析者")。

#### Story 2.1 — 选择简历来源(库内选 / 导入新简历)

- **作为**:拥有简历查看权限的用户
- **我想**:在"简历解析"页选择简历来源
- **以便**:对库内已有简历或新导入的简历触发解析
- **触发场景**:进入"简历解析"页,默认"从简历库选择"
- **AC**:
  - Given 我进入"简历解析"页,Then 左侧卡片展示两个分段选择:「从简历库选择」/「导入新简历」
  - And 默认选中"从简历库选择"
  - When 我点"从简历库选择",Then 下方 List 当前用户 Resume.List 权限范围内的简历
  - And 列表项展示:姓名、意向岗位、当前部门(从 DepartmentResume 反查);来源为"推荐"时附加"· 推荐"
  - When 我点"导入新简历",Then 切换为上传 PDF 区域(虚线框 + "点击上传简历")
- **边界用例**:
  - 当前渠道 + 权限范围内无可选简历:列表区显示"该渠道下暂无可选简历,请导入"
  - 切换来源模式后已选简历/已生成结果自动清空

#### Story 2.2 — 切换渠道(社招 / 校招)

- **作为**:拥有简历查看权限的用户
- **我想**:在"简历解析"页顶部切换社招 / 校招
- **以便**:分别处理不同渠道的简历
- **触发场景**:进入"简历解析"页,顶部 tab
- **AC**:
  - Given 我在"简历解析"页,Then 顶部展示两个 tab:「社招 SOCIAL」/「校招 CAMPUS」
  - And 默认选中"社招"
  - When 我点击另一渠道,Then 当前渠道切换,简历列表按新渠道 + 用户权限范围重新过滤
  - And 已选简历、已生成的解析结果自动清空
- **边界用例**:
  - 用户仅命中 `Resume.List attributeConditions.chan=social` 的权限:仅显示社招 tab,校招 tab 隐藏或置灰
  - 用户仅命中 `Resume.List attributeConditions.chan=campus` 反之

#### Story 2.3 — 选择目标岗位 JD

- **作为**:拥有简历查看 + 岗位查看权限的用户
- **我想**:在已选简历后,从下拉中选目标岗位 JD
- **以便**:基于该 JD 计算匹配度
- **触发场景**:已选简历,准备触发解析
- **AC**:
  - Given 我已选简历,Then "目标岗位 JD"下拉 List 当前用户 Position.List 权限范围内**且 status=on** 的岗位
  - And 下拉项格式:`{部门} · {岗位名}({渠道})`
  - When 我切换岗位,Then 已生成的解析结果自动清空
- **边界用例**:
  - 已下架岗位不出现在下拉
  - 不同渠道的岗位在下拉中通过 `(社招)` / `(校招)` 后缀明确区分
  - 无任何在架岗位(在用户权限范围内):下拉为空,提示"请先在「部门与岗位管理」中维护岗位"

#### Story 2.4 — 触发解析并查看匹配结果

- **作为**:拥有简历查看 + 岗位查看 + PositionResume.Create 权限的用户
- **我想**:点击"开始解析"按钮,等待系统给出与所选 JD 的匹配分析
- **以便**:判断候选人是否适合推进
- **触发场景**:已选简历 + 已选岗位,准备解析
- **AC**:
  - Given 我已选简历和岗位,When 点击"开始解析",Then 右侧结果区展示 AI 解析动画(scanner + 扫描光束 + `Thinking...` 进度文案)
  - And 解析完成后展示完整结果卡片;耗时由 PDF 解析、结构化抽取和匹配计算耗时决定
  - And 系统执行:
    - **Create PositionResume**(positionId, resumeId, kind=parsed, matchScore=计算结果)
    - List 候选人 vs JD 的关键词命中/未命中详情(Get 用于展示)
  - And 结果卡片包含:
    - 候选人头部:姓名、渠道标签、意向岗位、当前部门(从 DepartmentResume 反查)、来源、关键词 chip
    - 匹配度环图(总分 0-100,带颜色:绿≥80 / 琥珀≥65 / 红<65)
    - 系统判断文本(强烈推荐 / 建议进入面试 / 谨慎或暂不推荐)
    - 三项分数条:技能匹配、经验匹配、隐性要求
    - 关键词命中区:JD 关键词 vs 候选人关键词,命中用绿 chip + ✓,未命中用灰 chip
    - 隐性标签命中区:岗位隐性标签 vs 候选人 traits,同上配色
    - 文末"分析"提示框:说明技能命中数、隐性命中数、推进建议
  - And 解析结果卡片内的岗位字段一律用选中的 JD,不取简历中 pos 字段
- **匹配算法**(基础验收口径):
  - 技能匹配 = JD 关键词 ∩ 候选人关键词 / JD 关键词总数 × 100
  - 隐性要求 = (命中标签权重之和 / 总权重) × 100
  - 经验 = `expBase`(默认 60-90)
  - 总分 = 技能 × 0.4 + 经验 × 0.25 + 隐性 × 0.35,四舍五入到 0-100
- **边界用例**:
  - 候选人关键词为空:技能匹配 0,解析仍可完成
  - JD 隐性标签为空:隐性要求按 0 处理
  - 解析过程中再次点击"开始解析":忽略或重新计算均可,最终结果以最后一次为准

#### Story 2.5 — 生成三轮面试题

- **作为**:拥有 Resume.List 权限的用户
- **我想**:在解析结果卡片内点击"生成面试题"
- **以便**:按"专业 / 主管 / 资格"三类获得针对该候选人 + 该岗位的面试题
- **触发场景**:解析结果已展示,准备生成面试题
- **AC**:
  - Given 解析结果已展示,When 点击"生成面试题",Then 卡片下方"面试题"区域先展示 AI 生成状态(`Thinking...`),完成后出现题目列表
  - And 默认展示"专业面试"tab,顶部三个内嵌 tab:专业 / 主管 / 资格
  - And 每个题目展示:序号、题干、出题意图(why)、难度标签(核心/进阶/拔高/行为/动机/合规/流程)
  - And "主管面试"tab 的"为什么选择本部门"题,自动带入当前 JD 所属部门名(从 DepartmentPosition 反查)
  - And 经验值 ≥ 82 的高潜候选人,专业面试题加测一道拔高题
- **题型规则**:
  - 专业题至少 3 道,以候选人首个关键词或岗位关键词为引子提问
  - 主管题 3 道:跨团队协作、动机与稳定性、与上级意见不一致时的处理
  - 资格题 3 道:背调前置确认、薪酬/到岗确认、出差/加班接受度确认
- **边界用例**:
  - 切换 JD / 候选人后已生成的面试题自动清空

#### Story 2.6 — 导出面试题为 Markdown

- **作为**:拥有 Resume.List 权限的用户
- **我想**:在面试题区域点击"导出 MD"
- **以便**:把题目带走用于面试现场或存档
- **触发场景**:面试题已生成
- **AC**:
  - Given 面试题已生成,When 点击"导出 MD",Then 浏览器下载 `{候选人姓名}_面试题.md`
  - And 文件包含:H1 标题、岗位/部门/匹配度元信息、三章节(专业/主管/资格)、每题编号 + 题干 + 考察点
- **底层操作**:List PositionResume + List 候选人 + 格式转换(读取类操作)
- **边界用例**:
  - 浏览器拦截下载:提示用户允许下载

---

### Epic 3: 简历推荐 / 智能分流

> 入口:左侧导航"简历推荐"。依赖:E4 提供简历,E5 提供在架岗位。
> **访问条件**:用户拥有 Resume.List + Resume.Get + Resume.Create + Notification.Create + DepartmentResume.Create + PositionResume.Create 权限。

#### Story 3.1 — 触发智能分流

- **作为**:拥有简历推荐权限的用户
- **我想**:选择一份简历并点击"智能分流"
- **以便**:让系统给出该简历在各部门/岗位之间的最佳匹配去向
- **触发场景**:进入"简历推荐"页
- **AC**:
  - Given 我进入"简历推荐"页,Then 左侧卡片同样支持"从简历库选择 / 导入新简历",机制同 Story 2.1
  - And 顶部支持社招/校招渠道切换,机制同 Story 2.2
  - When 我选了简历,点击"智能分流",Then 右侧结果区展示 AI 分流计算状态(`Thinking...` + 轻量扫描/推理动效),完成后展示分流结果
- **边界用例**:
  - 当前渠道无在架岗位(用户权限范围内):展示空状态"该渠道下暂无在架岗位 / 请在「部门与岗位管理」中上架岗位"

#### Story 3.2 — 查看部门分流结果

- **作为**:拥有简历推荐权限的用户
- **我想**:看到一份按部门聚合的分流列表,每项含匹配分、推荐岗位、HRBP / 主管 / 锻炼干部
- **以便**:快速判断把这份简历推荐到哪个部门
- **触发场景**:智能分流完成
- **AC**:
  - Given 分流完成,Then 结果卡片展示分流列表(每部门取该部门下最高分岗位)
  - And 按总分降序排列,首位标记为"最佳去向"(绿色描边 + 浅绿背景)
  - And 每行展示:匹配分(大号数字,按 80/65 阈值上色)、部门名、推荐岗位、HRBP 姓名、主管姓名、锻炼干部姓名(多个人以顿号分隔)
  - And HRBP 信息从 UserDepartmentRole 推导:`HRBP` = 拥有 HRBP 角色 + 该部门 departmentId 绑定的用户;主管 / 锻炼干部同理
  - And 每行右侧按钮:最佳去向为蓝色"推荐到",其他为灰色"推荐到"
- **算法说明**:对所有在架且同渠道、且用户 Position.List 权限范围内的岗位算总分 → 按部门聚合取最高 → 部门间按最高分降序排列。

#### Story 3.3 — 推荐到目标部门并通知相关数据权限用户

- **作为**:拥有简历推荐权限的用户
- **我想**:点击某部门行的"推荐到"按钮
- **以便**:把这份简历归属到目标部门,并通知所有对该目标部门简历有数据权限的人
- **触发场景**:分流结果展示
- **AC**:
  - Given 我看到分流列表,When 点击某行的"推荐到",Then 系统执行以下 CRUD 组合:
    1. **List Resume** WHERE normalizedName=候选人姓名 AND 通过 DepartmentResume 关联到 target.departmentId
    2. 若不存在目标部门副本:**Create Resume**(副本,`source=推荐`, `sourceBy={我的姓名}({我的角色 label})`, 与原简历相同 attributes/pos/keywords/traits/expBase, createdAt=当前时间)
    3. 若不存在目标部门副本:**Create DepartmentResume**(副本.resumeId, target.departmentId, by=推荐人)
    4. 若已存在目标部门副本:**Update Resume**(sourceBy=新推荐人,保留原 resumeId)
    5. **Create/Update PositionResume**(resumeId, target.positionId, kind=recommended, by=推荐人;同 resumeId+positionId+kind 已存在时更新 by/createdAt)
    6. **List UserDepartmentRole + Permission** 计算通知收件人:对 target.departmentId 下该 Resume 满足 `Resume.List` 或 `Resume.Get` 权限的所有用户
    7. **Create N 条 Notification**(to=receiver.userId, resumeId, departmentId, positionId, name=候选人, by=推荐人, chan=Resume.attributes.chan)
    8. 若收件人包含我自己,发给自己的通知 `read=true`(自己推荐给自己不打扰)
  - And toast 提示"已推荐到「{部门名}」· 已通知 {N} 位相关人员"
  - And 若我本人也是收件人,我的顶部铃铛未读数不增加
- **边界用例**:
  - 同一份简历对同一部门重复"推荐到":**按方案 A(推荐去重)—— Create 前先 List Resume WHERE normalizedName=候选人姓名 AND 通过 DepartmentResume 关联到该部门,若已存在则 Update sourceBy=新推荐人,不重复 Create Resume / DepartmentResume**(参见 6.2)

---

### Epic 4: 简历库

> 入口:左侧导航"简历库"。依赖:E5 提供部门归属。
> **访问条件**:用户拥有 Resume.List 权限。

#### Story 4.1 — 查看简历列表(按 UserDepartmentRole 范围)

- **作为**:拥有 Resume.List 权限的用户
- **我想**:在简历库看到自己有权限查看的简历列表
- **以便**:了解当前手头的候选人池
- **触发场景**:进入"简历库"
- **AC**:
  - Given 我进入简历库,Then 默认显示当前渠道(社招)的列表
  - And 表格列:候选人姓名、年龄、毕业学校、工作年限、当前部门(从 DepartmentResume 反查)、意向岗位、来源、关键词、操作
  - And 表格不展示头像或头像首字母
  - And 操作列提供"查看详情"
  - And **列表过滤规则**:List Resume WHERE:
    - DepartmentResume.departmentId 在用户的 UserDepartmentRole.departmentId 范围内 **OR**
    - 用户命中的 Resume Permission 存在 `attributeConditions.chan` 时,Resume.attributes.chan 必须落在该属性条件范围内(系统级角色如社招负责人)
    - 多条 UserDepartmentRole 的允许范围按并集(OR)合并
  - And 表格右上角始终展示当前 UserDepartmentRole 集合的数据范围提示横幅(data-range-banner)
- **边界用例**:
  - 当前渠道无简历(在权限范围内):表格居中显示"该渠道下暂无简历"
  - 搜索无结果:显示"该渠道下暂无简历(无匹配关键词)"

#### Story 4.2 — 查看简历结构化详情

- **作为**:拥有 Resume.Get 权限的用户
- **我想**:在简历库点击某条简历的"查看详情"
- **以便**:查看候选人的结构化简历信息
- **触发场景**:简历库表格操作列
- **AC**:
  - Given 我对该简历有 Resume.Get 权限,When 点击"查看详情",Then 打开详情抽屉或详情页并 **Get Resume**
  - And 详情头部展示:候选人姓名、年龄、毕业学校、工作年限、当前部门、意向岗位、来源、来源人、创建时间、是否过期
  - And 详情主体展示结构化简历信息:基础信息、教育经历、工作经历、项目经历、技能关键词、证书/奖项(如有)
  - And 结构化字段为空时展示"未解析到",不隐藏整个模块
  - And 无 Resume.Get 权限时,"查看详情"按钮隐藏或置灰;直接访问详情 URL 时后端返回 403
- **底层操作**:Get Resume

#### Story 4.3 — 切换渠道与计数

- **作为**:拥有 Resume.List 权限的用户
- **我想**:在简历库切换社招 / 校招,并看到每个渠道的简历数
- **以便**:了解各渠道存量
- **触发场景**:简历库顶部 tab
- **AC**:
  - Given 我在简历库,Then 顶部展示「社招 {N}」/「校招 {M}」两个 tab,N/M 为当前权限范围 + 渠道过滤后的简历数
  - And 默认选中"社招"
  - When 我切换渠道,Then 列表刷新为该渠道(权限范围内)简历
- **边界用例**:
  - 用户仅命中 `Resume.List attributeConditions.chan=campus`:仅看到校招 tab,社招 tab 不可见或置灰
  - 用户仅命中 `Resume.List attributeConditions.chan=social`:反之

#### Story 4.4 — 关键词搜索

- **作为**:拥有 Resume.List 权限的用户
- **我想**:在搜索框输入关键词
- **以便**:按姓名 / 岗位 / 关键词命中检索
- **触发场景**:简历库顶部搜索框
- **AC**:
  - Given 我在搜索框输入文字,Then List Resume 时实时过滤(无须回车),在原权限过滤结果之上进一步筛选
  - And 过滤字段:候选人姓名、意向岗位、候选人关键词数组(任一命中即匹配)
  - And 命中的关键词在表格中以黄色 `<mark>` 高亮
  - And 搜索框始终保持焦点和光标位置(连续输入体验)
- **底层操作**:List Resume(应用权限过滤 + 关键词过滤)
- **边界用例**:
  - 输入为空:展示权限范围内全部
  - 大小写不敏感
  - 特殊字符:不作为正则元字符处理(防注入)

#### Story 4.5 — 单份导入(从解析/推荐页)

- **作为**:拥有 Resume.Create + DepartmentResume.Create 权限的用户
- **我想**:在解析或推荐页"导入新简历"
- **以便**:把新收到的 PDF 简历进入系统
- **触发场景**:解析/推荐页选择"导入新简历"模式
- **AC**:
  - Given 我在解析/推荐页切到"导入新简历",Then 展示 PDF 上传区域(虚线框)
  - And 提示文案:"支持 PDF · 单文件 ≤ 10MB · 猎头/猎手推荐"
  - When 我点击上传区,Then 触发文件选择并上传真实 PDF,导入完成后解析候选人信息
  - And 若当前用户只有一个非 system 部门绑定,该部门默认为导入目标部门
  - And 若当前用户有多个非 system 部门绑定或仅有 system 部门绑定,上传前必须选择"导入目标部门"
  - And 导入成功后,系统执行:
    - **Create Resume**(name, attributes.chan, attributes.expired=false, keywords, traits, expBase, source=导入, sourceBy=session.user.name, createdAt=当前时间)
    - **Create DepartmentResume**(resumeId, target.departmentId, by=session.user)
  - And 上传区下方绿色提示"✓ 已导入「{姓名}」并加入简历库"
- **底层操作**:Create Resume + Create DepartmentResume
- **边界用例**:
  - 文件 > 10MB:拒绝并提示
  - 非 PDF:拒绝并提示
  - 解析失败:提示并保留上传入口
  - 未能确定 target.departmentId:阻止导入并提示"请选择导入目标部门"

#### Story 4.6 — 批量导入

- **作为**:拥有 Resume.Create + DepartmentResume.Create 权限的用户
- **我想**:在简历库点击"批量导入简历"
- **以便**:一次性入库多份简历
- **触发场景**:简历库顶部"批量导入简历"按钮
- **AC**:
  - Given 我在简历库,When 点击"批量导入简历",Then 先选择或确认导入目标部门,再选择多份真实简历文件并按当前渠道批量入库
  - And 每份文件解析成功后:Create Resume + Create DepartmentResume(target.departmentId)
  - And toast 提示"已批量导入 {N} 份{渠道}简历"
- **底层操作**:N × (Create Resume + Create DepartmentResume)
- **备注**:批量导入采用"选择多文件上传 + 后台队列解析";生产系统不内置示例简历

#### Story 4.7 — 删除简历

- **作为**:拥有 Resume.Delete 权限的用户(且删除目标在权限范围内)
- **我想**:在简历库表格行尾点击"删除"
- **以便**:移除无效或误录的简历
- **触发场景**:简历库表格行尾
- **AC**:
  - Given 我有权限删除该简历(通过 DepartmentResume 反查的 departmentId 在我的 UserDepartmentRole.departmentId 范围内),When 点击行尾"删除",Then 系统执行:
    - **Delete Resume**(级联 Delete DepartmentResume / PositionResume 关联)
    - toast 提示"已删除该简历"
  - And 无权限的行:删除按钮置灰或隐藏
- **边界用例**:
  - 删除后:通知列表中关联此简历的旧通知保留(Notification 不级联删除,历史留痕)

#### Story 4.8 — 来源标识(导入 / 推荐)

- **作为**:浏览简历库的用户
- **我想**:清楚看到每条简历是由谁导入或推荐
- **以便**:判断简历的可信度与流转路径
- **触发场景**:简历库表格"来源"列
- **AC**:
  - Given 我看到一条简历,When `source=导入`,Then 来源列显示灰色 chip"`{sourceBy}`导入"
  - When `source=推荐`,Then 来源列显示青色 chip"`{sourceBy}`推荐"
  - And `sourceBy` 记录导入人或推荐人的姓名 / 角色摘要
  - And `createdAt` 与 `attributes.expired` 不在表格中单独成列,在 Story 4.2 的详情头部展示

---

### Epic 5: 部门与岗位管理

> 入口:左侧导航"部门与岗位管理",页面分为两个一级 tab: **部门管理**、**岗位管理**。
> **访问条件**:
> - 查看:拥有任一业务角色的用户可 List/Get Department、List DepartmentPosition、List/Get Position;游客仅可在简历解析页空状态中 List Department
> - 写入:只有**超级管理员**默认拥有 Department.Create/Update/Delete、Position.Create/Update/Delete、DepartmentPosition.Create/Update/Delete
> - HRD / HRBP / 主管 / 锻炼干部 / 社招负责人 / 校招负责人在本模块内仅可查看部门及岗位数据,不展示新增、编辑、删除、上下架入口
> - 部门的 HRBP / 主管 / 锻炼干部姓名不在本模块展示;仅在 Story 3.2 推荐分流结果中展示

#### Story 5.1 — 查看部门列表与详情

- **作为**:拥有 Department.List 权限的用户
- **我想**:在"部门管理"中查看部门清单和部门详情
- **以便**:了解当前系统可用的部门数据
- **触发场景**:进入"部门与岗位管理"默认 tab
- **AC**:
  - Given 我进入"部门管理",Then 表格展示所有我有权限查看的部门:部门名称、关联岗位数、关联简历数、更新时间、操作
  - When 我点击某个部门,Then 展示部门详情(Get Department):部门名称、关联岗位列表摘要、关联简历数
  - And 部门列表和详情**不展示**HRBP / 主管 / 锻炼干部人员清单
  - And 非超级管理员只看到"查看"操作;超级管理员额外看到"新增部门"、"编辑"、"删除"
- **底层操作**:List Department + Get Department + List DepartmentPosition + List DepartmentResume(计数)

#### Story 5.2 — 新增部门

- **作为**:超级管理员
- **我想**:在"部门管理"中点击"+ 新增部门"
- **以便**:接入新的业务部门
- **触发场景**:部门管理页上方
- **AC**:
  - Given 我是超级管理员,When 点击"+ 新增部门",Then 弹出新建表单(部门名称)
  - When 保存,Then 系统执行 **Create Department**(name=部门名称)
  - And 新部门保存后立即可用于岗位归属选择、简历导入目标部门选择、推荐分流部门聚合
  - And 本 Story 不创建 UserDepartmentRole,不配置 HRBP / 主管 / 锻炼干部;人员角色绑定仍在 Epic 6 用户与角色管理中完成
- **边界用例**:
  - 部门名称为空或重复:阻止提交并提示
  - 非超级管理员直接调用接口:后端返回 403

#### Story 5.3 — 编辑与删除部门

- **作为**:超级管理员
- **我想**:编辑或删除已有部门
- **以便**:维护部门主数据
- **触发场景**:部门管理表格行尾
- **AC**:
  - Given 我是超级管理员,When 点击"编辑",Then 弹出编辑表单,可修改部门名称
  - When 保存,Then 系统执行 **Update Department**(name)
  - Given 我是超级管理员,When 点击"删除",Then 二次确认后执行 **Delete Department**
  - And 删除部门前必须校验:若仍有关联岗位(DepartmentPosition)、归属简历(DepartmentResume)或用户角色绑定(UserDepartmentRole),则禁止删除并提示先解除关联
  - And 非超级管理员不展示编辑 / 删除按钮;直接调用接口返回 403
- **底层操作**:Update Department / Delete Department

#### Story 5.4 — 查看岗位列表与 JD 详情

- **作为**:拥有 Position.List 权限的用户
- **我想**:在"岗位管理"中查看岗位列表和 JD 详情
- **以便**:了解岗位职责、硬性要求、关键词、隐性标签和状态
- **触发场景**:进入"岗位管理"tab 或从解析 / 推荐场景查看岗位
- **AC**:
  - Given 我进入"岗位管理",Then 岗位列表按部门分组或按部门筛选,每项展示:岗位名、所属部门、渠道 chan 标签、职级、状态(在架/下架)、关键词数、隐性标签数
  - When 我点击某个岗位,Then 右侧 JD 详情展示(Get Position):
    - 头部:岗位名、部门(从 DepartmentPosition 反查)、职级、渠道标签、状态(在架/下架)
    - 岗位职责列表
    - 硬性要求列表
    - 匹配关键词(蓝色 chip)
    - 隐性要求标签与权重(权重条 + 百分比)
    - 状态说明
  - And 非超级管理员只读查看;超级管理员额外看到"+ 新增岗位"、"编辑"、"上架 / 下架"、"删除"
- **底层操作**:List Position + Get Position + List DepartmentPosition
- **边界用例**:
  - 已下架岗位:仍可选中查看,但不参与解析与分流

#### Story 5.5 — 新增与编辑岗位

- **作为**:超级管理员
- **我想**:新增岗位或编辑岗位 JD 信息
- **以便**:维护岗位主数据和匹配规则
- **触发场景**:岗位管理页上方"+ 新增岗位"或岗位详情头部"编辑"
- **AC**:
  - Given 我是超级管理员,When 点击"+ 新增岗位",Then 弹出岗位表单:岗位名、所属部门、渠道 chan、职级、状态、岗位职责、硬性要求、匹配关键词、隐性标签与权重
  - When 新增保存,Then 系统执行:
    - **Create Position**(name, chan, level, status, duties, must, keywords, implicit)
    - **Create DepartmentPosition**(departmentId, positionId)
  - Given 我是超级管理员,When 点击"编辑",Then 表单预填当前岗位数据
  - When 编辑保存,Then 系统执行 **Update Position**(可变字段)
  - And 若所属部门变化,系统同步更新 DepartmentPosition(删除旧关联并创建新关联,或 Update DepartmentPosition)
  - And 关键词和隐性标签重复时拒绝保存;隐性标签默认权重为 40%(保存为 `w: 40`),权重和不需要等于 100(算法自动归一化,见 Story 2.4)
  - And 非超级管理员不展示新增 / 编辑入口;直接调用接口返回 403
- **边界用例**:
  - 已展示的解析结果不自动回算;下次重新解析时使用新 JD

#### Story 5.6 — 岗位上下架与删除

- **作为**:超级管理员
- **我想**:上架、下架或删除岗位
- **以便**:控制岗位是否参与解析分流,并清理无引用岗位
- **触发场景**:岗位详情头部或岗位列表行尾
- **AC**:
  - Given 我是超级管理员,When 当前岗位 `status=on`,Then 显示"下架"按钮;反之显示"上架"
  - When 点击切换,Then **Update Position**(status=on/off),toast 提示"岗位已上架/下架"
  - And 下架后:该岗位不出现在 Story 2.3 的 JD 下拉、不出现在 Story 3.1 的分流计算
  - And 下架岗位在岗位列表标"已下架"(降低不透明度),仍可查看 JD 详情
  - Given 我是超级管理员,When 点击"删除",Then 二次确认后执行 **Delete Position** + **Delete DepartmentPosition**
  - And 若岗位已存在 PositionResume 解析 / 推荐历史,禁止物理删除并提示使用"下架"
  - And 非超级管理员不展示上下架 / 删除入口;直接调用接口返回 403

---

### Epic 6: 用户与角色管理

> 入口:左侧导航"用户管理"。
> **访问条件**:
> - Get User(self):任一已登录用户
> - List User + List UserDepartmentRole:拥有 HRD / 超级管理员 / 对应自定义管理角色
> - Create/Delete UserDepartmentRole:超级管理员或被授予该 Permission 的自定义角色

#### Story 6.1 — 查看用户列表

- **作为**:拥有 User.List 权限的用户
- **我想**:在用户管理看到所有用户及其角色
- **以便**:了解组织人员与角色分配
- **触发场景**:进入"用户管理"
- **AC**:
  - Given 我进入该页,Then 表格列:姓名、工号、**当前角色集合(含部门)**、所属部门、操作
  - And "当前角色集合"列展示:从 UserDepartmentRole List 得出,每条格式 `{role.label}(部门:{dept.name})`,多条以竖线或换行分隔
    - 例:`HRBP(部门:算力训练平台部、智算调度部) | 主管(部门:硬件加速部)`
  - And 无任何 UserDepartmentRole 业务绑定的用户(仅游客):展示 `游客` chip
  - And 表格上方仅展示拥有 UserDepartmentRole.Create 权限时可用的角色分配入口;其他用户看到只读提示横幅
- **底层操作**:List User + List UserDepartmentRole + List Department
- **边界用例**:
  - 搜索"姓名 / 工号 / 部门"实时过滤,命中关键词高亮(机制同 Story 4.4)

#### Story 6.2 — 给用户分配角色(单条绑定)

- **作为**:拥有 UserDepartmentRole.Create 权限的用户
- **我想**:点击某用户行尾的"分配角色"
- **以便**:给该用户授予一个角色及部门
- **触发场景**:用户列表行尾(操作列)
- **AC**:
  - Given 我有 `UserDepartmentRole.Create`,When 点击"分配角色",Then 弹出"角色绑定"弹窗
  - And 弹窗包含:
    - 用户信息(姓名、工号,只读)
    - 角色选择下拉:**列出全部 `enabled=true` 的角色**(预置 + 自定义),按"系统角色 / 自定义角色"分组,每条展示角色 label 与 Resume 属性条件摘要
    - **部门选择**:List Department 全集,必选;系统级角色(如超级管理员 / 社招负责人 / 校招负责人 / 全部门 HRD)选择 system 部门
    - "已有绑定"列表:List UserDepartmentRole 展示该用户当前所有绑定(含游客),每条带"移除"按钮
  - When 我选好角色与部门并保存,Then **Create UserDepartmentRole**(userId, departmentId, roleId)
  - And 用户的有效权限立即生效(下次 API 调用或下次进入页面)
- **边界用例**:
  - 重复绑定(同 userId+departmentId+roleId):阻止并提示
  - 给已有该角色的用户再加新部门(department 不重叠):允许,创建为新 UserDepartmentRole
  - 给纯游客用户首次分配业务角色:游客 UserDepartmentRole **保留**(便于审计),与新业务绑定并存

#### Story 6.3 — 给用户分配多个角色(聚合)

- **作为**:拥有 UserDepartmentRole.Create 权限的用户
- **我想**:对同一用户连续添加多条 UserDepartmentRole(不同角色或同角色不同部门)
- **以便**:体现"有全部数据权限的用户可分配多个"的能力,以及业务一人多角的真实情况
- **触发场景**:Story 6.2 弹窗内
- **AC**:
  - Given 我在"角色绑定"弹窗,When 重复点击"添加另一绑定",Then 可继续选角色 + 部门,加入"待添加"列表
  - And "待添加"列表逐条展示,可单独移除
  - When 一次性保存,Then 系统 **Create** 多条 UserDepartmentRole(支持原子事务:全部成功或全部失败)
  - And 用户当前有效权限 = 所有 UserDepartmentRole 通过 RoleRelation 展开后所有 Permission 的并集
- **典型场景**:
  - 张敏 = UserDepartmentRole(张敏, 算力训练平台部, HRBP) + UserDepartmentRole(张敏, 智算调度部, HRBP) + UserDepartmentRole(张敏, 硬件加速部, 主管) → 可看 3 个部门的简历
  - 李建国 = UserDepartmentRole(李建国, system 部门, HRD) → 自动获得全 HRD 范围(因 HRD 默认绑定到 system 即覆盖全部门)
  - 业务临时需求:UserDepartmentRole(用户, dept:X, HRBP) + UserDepartmentRole(用户, dept:X, 自定义"高级评审者")→ 聚合后该用户在 X 部门同时有 HRBP 和高级评审权限
- **边界用例**:
  - 同一用户出现重复等效 UserDepartmentRole(同 user+dept+role):保存前提示合并或阻止
  - 角色绑定上限:暂不限制,业务自行评估

#### Story 6.4 — 解除用户角色绑定

- **作为**:拥有 UserDepartmentRole.Delete 权限的用户
- **我想**:在 Story 6.2 的"已有绑定"列表点击某条 UserDepartmentRole 的"移除"
- **以便**:撤销用户某项权限
- **触发场景**:用户列表 → 分配角色弹窗内的"已有绑定"区
- **AC**:
  - Given 我有 `UserDepartmentRole.Delete`,When 点击某条 UserDepartmentRole 旁的"移除",Then 系统提示"确认解除 {role.label}(部门:{dept.name})?",二次确认后 **Delete UserDepartmentRole**
  - And 用户该部门范围内的权限立即失效
- **边界用例**:
  - 解除后该用户**仅剩游客 UserDepartmentRole**:用户回退为游客状态(Story 1.2)
  - 解除后该用户**无任何 UserDepartmentRole(连游客都没有)**:系统提示"是否恢复游客身份?",确认后自动 Create 一条游客 UserDepartmentRole
  - 不能解除自己的所有业务绑定(防锁出):若勾选全部,阻止并提示"至少保留一条业务绑定或游客身份"

---

### Epic 7: 通知与提醒

> 横切能力:由 E3 推荐行为触发,入口在顶部铃铛。

#### Story 7.1 — 铃铛未读数显示

- **作为**:任一已登录用户
- **我想**:在顶部铃铛上看到未读通知数
- **以便**:知道是否有新简历被推荐到我有数据权限的部门
- **触发场景**:登录后任意页面
- **AC**:
  - Given 我已登录,Then 顶部铃铛右侧红色圆点展示我的未读通知数
  - And 无未读时圆点隐藏
  - And 侧边导航"简历库"项右侧也展示同样的未读徽章
- **底层操作**:List Notification WHERE to=session.id AND read=false

#### Story 7.2 — 查看通知列表

- **作为**:拥有 Notification.List 权限的用户
- **我想**:点击铃铛看到通知列表
- **以便**:了解是谁把哪份简历推荐到了我可查看的部门
- **触发场景**:顶部铃铛按钮
- **AC**:
  - Given 我有点击铃铛,Then 弹出下拉框,顶部标题"推荐提醒(N 条未读)"
  - And 每条通知展示:头像、`<姓名> 被推荐到「{部门}」`、`<推荐人> · <时间>`
  - And 无未读时,显示空状态"暂无新的推荐提醒"

#### Story 7.3 — 标记全部已读

- **作为**:拥有 Notification.Update 权限的用户
- **我想**:在通知顶部点击"全部已读"
- **以便**:清空未读数
- **触发场景**:通知下拉框顶部
- **AC**:
  - Given 我有未读通知,When 点击"全部已读",Then **批量 Update Notification**(read=true) WHERE to=session.id
  - And 铃铛圆点与侧边导航徽章立即消失
  - And toast 提示"已全部标记为已读"

#### Story 7.4 — 点击通知跳转

- **作为**:拥有 Notification.Update + Resume.List 权限的用户
- **我想**:点击单条通知
- **以便**:直接进入简历库查看该候选人
- **触发场景**:通知下拉框某条
- **AC**:
  - Given 我点击某条通知,Then **Update Notification**(read=true),下拉框关闭
  - And 自动跳转到"简历库"页,渠道根据被推荐简历的 `attributes.chan` 自动切换(社招切社招,校招切校招)
  - And 简历库顶部展示绿色横幅"有 N 份简历被推荐到你可查看的部门",列出姓名、部门与推荐人

---

### Epic 8: 角色管理(自定义)

> 入口:左侧导航"角色管理"(**仅超级管理员可见**)。
> 支持超级管理员按需创建、编辑、删除和启停角色;Permission / RoleRelation 直接表达角色"内容"。

#### Story 8.1 — 查看角色列表

- **作为**:超级管理员
- **我想**:在"角色管理"看到所有角色定义
- **以便**:了解当前权限模型,并按需扩展
- **触发场景**:进入"角色管理"
- **AC**:
  - Given 我是超级管理员,When 进入"角色管理"页,Then 表格 List 全部 Role(系统预置 + 自定义)
  - And 列:角色 label、类型(无,统一为"角色")、Permission 数(从 Permission 表 count)、RoleRelation 数(子角色数)、数据条件摘要(从 Permission.attributeConditions 汇总)、状态(启用/禁用)、引用计数(UserDepartmentRole 引用次数)、是否系统预置、操作
  - And 系统预置角色展示"系统"徽章,操作列只允许"编辑"和"启用/禁用",**不允许删除**
  - And 自定义角色操作列允许"编辑"、"删除"(有引用时禁用按钮)、"启用/禁用"
  - And 支持按"系统/自定义"、"启用/禁用"过滤;搜索框按 label 模糊匹配
- **底层操作**:List Role + List Permission + List RoleRelation + List UserDepartmentRole
- **边界用例**:
  - 角色列表默认按"系统 → 自定义,label 字母序"排序
  - 引用计数 = 0 时删除按钮才可点

#### Story 8.2 — 创建角色

- **作为**:超级管理员
- **我想**:点击"+ 新建角色"
- **以便**:定义新的角色
- **触发场景**:角色列表上方"+ 新建角色"
- **AC**:
  - Given 我是超级管理员,When 点击"+ 新建角色",Then 弹窗:
    - label(必填,全局唯一,长度 2-20)
    - description(可选)
    - **Permission 多选**:从产品白名单中勾选有效的 (resource, action) 组合;底层仍使用 12 个资源 × 5 个 CRUD 的矩阵表达,但 UI 不展示无业务意义或风险过高的组合
    - **Permission 条件**:对支持属性过滤的资源可配置 attributeConditions;当前 Resume 支持 `chan in {social, campus}` 与 `expired in {true,false}`
    - **包含角色(RoleRelation)多选**:列出已有 Role,可勾选若干作为子角色;勾选后该新角色会自动包含这些子角色的 Permission(递归展开)
  - When 保存,Then **Create Role**(label+description+isSystem=false+enabled=true),并按勾选的内容 **Create N 条 Permission(含 attributeConditions)** + **Create M 条 RoleRelation**
  - And 该角色立即出现在 Story 6.2 的角色下拉中,可被 UserDepartmentRole.Create 引用
- **底层操作**:Create Role + Create Permission (批量) + Create RoleRelation (批量)
- **边界用例**:
  - label 重复:阻止并提示
  - 包含自身或形成间接循环:阻止(避免 RoleRelation 递归展开死循环)

#### Story 8.3 — 编辑角色

- **作为**:超级管理员
- **我想**:点击某角色的"编辑"
- **以便**:调整 Permission / RoleRelation / 元数据
- **触发场景**:角色列表行尾
- **AC**:
  - Given 我是超级管理员,When 点击"编辑",Then 弹窗同 Story 8.2,字段预填当前值
  - And 系统预置角色:可编辑 description / enabled / Permission / RoleRelation;**label / isSystem 不可改**
  - And 自定义角色:可编辑所有字段
  - When 保存,Then 系统:
    - **Update Role**(可变字段)
    - 重新同步 Permission:Delete 全量旧 Permission + Create 新勾选集合
    - 重新同步 RoleRelation:Delete 全量旧 RoleRelation + Create 新勾选集合
  - And 已存在的 UserDepartmentRole **立即按新定义生效**(无需重新绑定)
- **底层操作**:Update Role + Delete+Create Permission + Delete+Create RoleRelation
- **边界用例**:
  - 编辑时移除某 Permission:所有依赖该 Permission 的用户,对应操作权限立即失效
  - 编辑时移除某 RoleRelation:依赖该子角色的间接权限失效
  - 编辑 RoleRelation 时形成直接或间接循环:阻止并提示"角色包含关系不能形成循环"

#### Story 8.4 — 删除自定义角色

- **作为**:超级管理员
- **我想**:点击某自定义角色的"删除"
- **以便**:清理不再使用的角色定义
- **触发场景**:角色列表行尾
- **AC**:
  - Given 我是超级管理员,且目标角色 `isSystem=false` 且引用计数 = 0,When 点击"删除",Then 二次确认后系统执行:
    - **Delete Role**(级联 Delete Permission + RoleRelation)
  - And 审计日志保留被删除角色的 roleId / roleLabel / 删除人 / 删除时间快照
  - And 该角色从 Story 6.2 的角色下拉中消失
- **底层操作**:Delete Role + Delete Permission + Delete RoleRelation
- **边界用例**:
  - 系统预置角色:删除按钮禁用
  - 引用计数 > 0:删除按钮禁用,提示"该角色被 N 个 UserDepartmentRole 引用,请先解除绑定"
  - 引用计数 = 0:允许删除,二次确认

#### Story 8.5 — 启用 / 禁用角色

- **作为**:超级管理员
- **我想**:点击某角色的"启用/禁用"切换
- **以便**:临时下线某角色(不影响已绑定用户,也不丢失定义)
- **触发场景**:角色列表行尾
- **AC**:
  - Given 我是超级管理员,When 点击"禁用",Then **Update Role**(enabled=false)
  - And 该角色从 Story 6.2 的角色下拉中消失
  - And **已绑定的 UserDepartmentRole 仍生效**(权限不撤销)
  - And 角色列表中标记为"已禁用"
  - When 再次点击"启用",Then **Update Role**(enabled=true),恢复出现在下拉中
- **边界用例**:
  - 禁用系统预置角色:允许,但需二次确认"禁用后将无法分配新角色,已有绑定继续生效"
  - 禁用一个被某其他角色通过 RoleRelation 引用的角色:允许,父角色运行时跳过该禁用项

---

## 5. 非功能需求

### 5.1 权限隔离

- 所有 API 必须在后端按用户的 UserDepartmentRole 展开后的权限集做权限校验
- 越权访问(手动改 URL / API 参数)由后端拦截,前端 403 提示并引导回主页
- UserDepartmentRole 解除后权限立即失效(无需重新登录)
- 校验流程参见 1.4 的 `user_can()` 算法

### 5.2 操作留痕

- 以下行为应写入审计日志:登录、解析、推荐、上下架、JD 编辑、隐性标签增删、UserDepartmentRole Create/Delete、W3 登录用户入库、Permission / RoleRelation 变更
- 字段建议:操作人(用户工号 + 当时生效的角色集合)、时间、操作类型、目标资源、变更前后值
- 审计日志为生产必备能力,不得只依赖前端展示状态

### 5.3 PDF 上传限制

- 单文件 ≤ 10MB
- 仅接受 PDF 类型
- 超出限制:前端预校验 + 后端二次校验,均拒绝

### 5.4 简历过期标记

- 简历包含 `attributes.expired` 字段,用于标记是否过期
- 系统不做自动过期计算;过期状态由导入、运营流程或管理功能更新
- 业务上"清理"仍通过 Delete Resume(Story 4.7)实现,不因 expired=true 自动删除

### 5.5 生产数据约束

- 生产系统不内置示例账号、示例简历、示例岗位或任何测试业务数据
- 生产页面只展示真实业务功能和真实业务数据
- 生产登录页只保留真实 W3 登录入口
- 评审、测试或培训所需数据只能存在于非生产环境,并与生产数据隔离

### 5.6 性能预期

- 解析、智能分流与面试题生成过程均展示 `Thinking...` AI 计算状态
- 系统耗时按实际模型、规则计算和文件解析耗时决定
- 列表页 / 表格 / 搜索:前端实时过滤,数据量 < 1000 时无明显卡顿

### 5.7 可访问性

- 字号、行高、颜色对比度满足可读性要求(对比度 ≥ 4.5:1)
- 全局支持 `prefers-reduced-motion`,用户开启减少动态效果时自动关闭非必要动画
- 关键交互元素可键盘聚焦;Tab 顺序合理

---

## 6. 通用业务规则与交互要求

### 6.1 上下架联动

- 下架岗位:
  - 不出现在 Story 2.3 的 JD 下拉(Position.List 自动过滤 status=on)
  - 不参与 Story 3.1 的分流计算
  - 仍可在 Story 5.2 查看 JD 详情(Get Position 不受 status 限制)
  - 在左侧岗位列表中以"已下架"tag 和降低不透明度标识

### 6.2 推荐去重策略

- **规则**:同一候选人推荐到同一部门时执行推荐去重
  - 同一份简历对同一部门重复"推荐到",按 `(normalizedName + 目标部门)` 去重
  - Story 3.3 步骤 1 的 Create Resume 前先 List Resume WHERE normalizedName=候选人姓名 AND 通过 DepartmentResume 关联到该部门
  - 若已存在副本:不 Create Resume / DepartmentResume,改 **Update Resume.sourceBy=新推荐人**,并 Create/Update PositionResume 关联目标岗位
  - 若不存在:Create 新副本 + Create DepartmentResume
- 实现细节:此去重逻辑在 API 层处理,数据库层通过 `(normalizedName + DepartmentResume.departmentId)` 联合查询实现;若系统具备身份证明或 PDF hash,可升级为更稳定的 candidateKey

### 6.3 空状态文案

| 场景 | 文案 |
|------|------|
| 解析结果区(未触发) | "解析结果将显示在这里 / 选择/导入简历并选岗位后点击「开始解析」" |
| 推荐结果区(未触发) | "分流结果将显示在这里 / 选择/导入简历后点击「智能分流」" |
| 简历库列表为空 | "该渠道下暂无简历" |
| 简历库搜索无结果 | "该渠道下暂无简历(无匹配关键词)" |
| 简历选择列表为空 | "该渠道下暂无可选简历,请导入" |
| 无在架岗位(用户权限范围内) | "该渠道下暂无在架岗位 / 请在「部门与岗位管理」中上架岗位" |
| 通知列表为空 | "暂无新的推荐提醒" |
| 游客可见业务页面 | "简历解析"、"简历推荐" |

### 6.4 搜索关键词高亮

- 在 Story 4.4 / 6.1 搜索场景,命中的关键词在表格中以黄色 `<mark>` 高亮
- 高亮范围:候选人姓名、关键词 chip
- 转义:高亮前先 HTML 转义防 XSS

### 6.5 数据范围提示横幅(data-range-banner)

- 简历库、用户管理页:当前 UserDepartmentRole 集合对应数据范围不完整时,顶部蓝色横幅说明
- 文案模板:"当前数据权限:{按 UserDepartmentRole 摘要,如 '负责部门:算力训练平台部、智算调度部'}"

### 6.6 角色权限只读模式

- 用户管理:无 `UserDepartmentRole.Create` 角色,操作列显示"只读"灰色文案,不出现编辑/分配
- 部门与岗位管理:非超级管理员为只读模式,部门管理不展示新增/编辑/删除,岗位管理不展示新增/编辑/上下架/删除

### 6.7 Toast 反馈统一

- 所有成功 / 提示操作均以底部居中 toast 提示,2.2s 自动消失
- 关键 toast 文案:
  - 登录成功:"已通过 W3 登录 · {角色 label 列表}"
  - 推荐成功:"已推荐到「{部门}」· 已通知 {N} 位相关人员"
  - 删除成功:"已删除该简历"
  - 批量导入:"已批量导入 {N} 份{渠道}简历"
  - 面试题生成:"已生成三轮面试题"
  - 标记已读:"已全部标记为已读"
  - 上下架:"岗位已上架/下架"
  - 角色分配:"已为 {姓名} 分配 {角色}(部门:{部门})"

### 6.8 字段空值处理

- 候选人 `pos=待定`(导入但未指定岗位时):解析 / 推荐仍可触发,但推荐结果以"无意向岗位"标识
- 候选人 `keywords[]` / `traits[]` 为空:技能匹配按 0 处理,解析仍可完成
- 来源人 `sourceBy` 为空:来源 chip 显示"未知来源",详情中的来源人字段显示"—"
- `DepartmentPosition` 缺失(岗位未关联部门):List Position 时该岗位不出现在按部门分组的岗位列表和推荐分流候选岗位中,JD 详情中部门字段显示"—"
- 导入简历时若无法确定 target.departmentId:必须要求用户选择目标部门,不能落到空部门

### 6.9 部门配置实时生效

- UserDepartmentRole / DepartmentPosition 变更后,下次进入或主动刷新岗位列表、JD 详情、推荐分流结果即生效
- 推荐分流结果中的 HRBP / 主管 / 锻炼干部姓名从最新 UserDepartmentRole 实时推导,仅在推荐场景展示
- 系统不提供单独的"查看部门关系对应表"入口

### 6.10 W3 登录用户入库策略

- 用户入库只由 W3 单点登录触发(`Story 1.1`);用户管理不提供"从 W3 同步"或手动批量同步入口
- W3 登录阶段先校验账号存在且员工在职;非在职 / 不存在账号不能进入本系统,也不创建 User
- W3 登录成功后写入 / 刷新的字段严格限定:**`id` + `name` + `employeeId`**;其他信息(邮箱、手机号、岗位等)不入本系统
- W3 认证或用户入库失败:登录失败,不创建半完成用户记录,记录日志并提示用户稍后重试

### 6.11 游客状态流转

**规则:游客 UserDepartmentRole 永久保留(不删除)**

| 场景 | 游客 UserDepartmentRole | 业务 UserDepartmentRole | 用户可见权限 |
|------|:----------------:|:----------------:|--------------|
| W3 首次登录(本系统无记录) | **Create** | 无 | 可见页面仅含简历解析 / 简历推荐;资源操作以后端权限为准 |
| 已分配业务角色后 | **保留**(审计追溯) | 至少 1 条 | 按业务绑定取最高权限 |
| 业务角色全部解除(超级管理员或授权管理角色手动) | **保留** | 0 条 | 回退为游客状态 |
| 用户完全无任何 UserDepartmentRole(异常态) | **自动 Create** | 无 | 回退为游客状态 |

**保留理由**:
- 审计追溯:何时创建、何时被分配业务角色、何时业务角色被解除,全链路可查
- 实现简单:无需在每次角色变更时做游客判断
- 防御异常:任何边界态下用户至少有一条"基础 UserDepartmentRole"

### 6.12 单设备登录 / 多端会话

- **会话策略**:启用"后登录踢前登录"
- 同一 W3 `id` 同一时间只保留一个有效会话
- 当同一用户在新设备 / 新浏览器登录成功后,系统立即使旧会话失效;旧会话下次请求返回未登录并引导回登录页
- 用户主动"退出登录 / 切换账号"时,仅清空当前会话

### 6.13 简历解析结果记录语义

- **记录规则**:不保存完整解析快照
- PositionResume 只记录本次解析关联与 `matchScore`,不保存当时 JD、隐性标签版本或完整计算明细快照
- 已展示的解析结果仅作为当前页面结果;用户下次重新解析时按最新 Resume / Position 数据重新计算
- 不新增 PositionResumeSnapshot 表

---

## 7. UI 设计规范

版本：v1.0  
适用对象：算力事业部招聘智能助手、招聘决策中枢、简历解析与候选人推进相关页面  
视觉基准：`visual-preview.html` 高端暗色风格

### 7.1 设计目标

本产品不是营销页，也不是普通后台管理系统。它应呈现为一个可信、克制、精密的企业级人才决策中枢。

核心受众：

- HRBP：快速判断候选人是否值得推进，并追踪证据链。
- 用人主管：查看岗位匹配、风险项和下一步面试动作。
- 管理员：确认权限边界、通知状态和候选人流转记录。

业务目标：

- 让“简历解析”从文档处理变成可复核的招聘决策。
- 在首屏回答：当前候选人是谁、是否推进、风险在哪里、下一步做什么。
- 保持高信息密度，但不让界面变成拥挤表格。

品牌气质：

- 高端、冷静、精密、有判断力。
- 像高可信企业智能系统，不像普通 OA 后台。
- 高级感来自层级、比例、材质、留白和真实业务状态，不来自堆叠光效。

### 7.2 视觉原则

#### 7.2.1 关键词

- 黑曜石底色
- 金属细线
- 冷青动作色
- 克制的宋体判断标题
- 精密数据网格
- 低噪声状态反馈
- 中高信息密度

#### 7.2.2 好看的判断标准

一个页面只有在满足以下条件时才算符合本规范：

- 首屏有一个明确的高层判断锚点，而不是只展示功能入口。
- 所有模块都服务招聘决策链：输入、解析、证据、风险、动作、权限。
- 动作色低频出现，只强调最关键操作和关键状态。
- 面板边界清晰，但不依赖厚重阴影或彩色边框。
- 中文标题有品牌气质，正文和数据保持可读。
- 所有按钮、输入、加载、空态、错误态都有明确反馈。

### 7.3 色彩系统

全部颜色优先使用 OKLCH 或语义 token。禁止在组件内随意新增高饱和颜色。

```css
:root {
  --bg: oklch(10% 0.018 245);
  --shell: oklch(13% 0.018 245);
  --panel: color-mix(in oklch, oklch(18% 0.018 245), transparent 10%);
  --panel-strong: oklch(20% 0.02 245);
  --fg: oklch(93% 0.01 240);
  --muted: oklch(66% 0.018 240);
  --faint: oklch(48% 0.018 240);
  --border: rgba(227, 238, 246, 0.13);
  --border-strong: rgba(227, 238, 246, 0.22);
  --accent: oklch(73% 0.13 190);
  --accent-soft: color-mix(in oklch, var(--accent), transparent 82%);
  --warn: oklch(76% 0.12 78);
  --danger: oklch(67% 0.16 25);
  --success: oklch(72% 0.13 156);
}
```

#### 7.3.1 使用比例

- 背景与深色面板：80% 以上。
- 白色文字与弱文字：10% 到 15%。
- 冷青动作色：每个屏幕最多 2 到 3 个强可见位置。
- 语义色只用于状态，不作为装饰色。

#### 7.3.2 背景

推荐背景结构：

- 底层使用 `--bg`。
- 叠加低透明度线性渐变，制造暗色材料层次。
- 使用细线网格表达“计算/决策系统”气质。
- 可以有一条克制的信号切面，但不能变成霓虹背景。

禁止：

- 大面积紫蓝渐变。
- 暖米色、桃粉、棕橙色背景。
- 无意义光团、光晕、流体 blob。
- 多个高饱和 accent 同屏竞争。

### 7.4 字体系统

```css
:root {
  --font-display: "Songti SC", "STSong", "Noto Serif SC", "Iowan Old Style", serif;
  --font-body: -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif;
  --font-mono: "SFMono-Regular", "JetBrains Mono", "IBM Plex Mono", ui-monospace, monospace;
}
```

#### 7.4.1 字体角色

- 显示字体：仅用于高层判断标题和少量品牌表达，不用于真实交互页顶部的大字宣言。
- 正文字体：用于导航、说明、卡片内容、按钮。
- 等宽字体：用于编号、指标、标签、分数、系统状态。

#### 7.4.2 字号建议

| 角色 | 尺寸 | 行高 | 字距 |
|---|---:|---:|---:|
| 工作台页面标题 | 24-32px | 1.2 | 0 |
| 高层判断标题 | 24-34px | 1.12-1.2 | -0.01em |
| 区块标题 | 16-20px | 1.3 | 0 |
| 正文 | 13-15px | 1.6-1.7 | 0 |
| 标签 / Eyebrow | 11-12px | 1.0-1.3 | 0.06-0.08em |
| 数字 / 分数 | 26-44px | 1.0 | -0.02em |

#### 7.4.3 排版规则

- 真实交互页面顶部只放任务型标题，例如“简历解析”“智能分流”“简历库”，不得使用宣言式大字文案。
- 42px 以上的展示标题仅允许用于非生产演示页、品牌封面或对外汇报封面，不进入日常业务交互界面。
- 高层判断标题可以使用显示字体，但字号必须克制，并且必须直接表达业务判断。
- 英文大写标签必须增加字距，建议 `0.08em`。
- 正文行宽控制在 62ch 以内。
- 重要数字使用 `font-variant-numeric: tabular-nums;`。
- 避免过度加粗；标题通常使用 520-560，按钮和导航使用 560。

### 7.5 布局系统

#### 7.5.1 页面结构

标准工作台布局：

```text
左侧导航栏
└─ 品牌标识
└─ 工作区导航
└─ 当前策略 / 权限提示

主内容区
└─ 顶部任务标题 + 快捷动作
└─ 今日决策信号带
└─ 三栏工作区
   ├─ 候选人输入
   ├─ 高层决策摘要
   └─ 权限与上下文
```

#### 7.5.2 栅格

- 桌面端主布局：`272px + minmax(0, 1fr)`。
- 工作区三栏：`0.82fr / 1.38fr / 0.8fr`，中间决策区必须最大。
- 栏间距：18px。
- 页面外边距：`clamp(22px, 3vw, 44px)`。
- 小屏在 1180px 以下改为单列。

#### 7.5.3 间距

| 场景 | 建议 |
|---|---:|
| 页面边距 | 22-44px |
| 卡片内边距 | 18-24px |
| 模块间距 | 18-30px |
| 按钮间距 | 10px |
| 列间距 | 18px |
| 列表项间距 | 12-14px |

### 7.6 组件规范

#### 7.6.1 左侧导航

视觉：

- 深色半透明栏。
- 右侧使用 1px 低透明边界。
- 当前项使用弱白底和浅边框，不使用彩色左边框。

交互：

- 导航项使用 `button`，不要使用 `href="#"` 假链接。
- 当前页面使用 `aria-current="page"`。
- hover 与 active 只改变边界、背景和文字亮度。

#### 7.6.2 品牌标识

品牌 mark 应是几何、克制、低装饰的系统符号。

规则：

- 尺寸：42px。
- 形状：方形或低圆角，不使用大圆角胶囊。
- 内部可用 45 度几何线表达“决策/匹配”。
- 不使用 emoji 或插画人物。

#### 7.6.3 决策信号带

用途：

在首屏快速回答当前招聘判断。

必备字段：

- 当前候选人
- 推进判断
- 权限边界
- 下一动作

视觉：

- 使用整条横向网格，不拆成四张漂浮卡片。
- 每个单元高度不低于 112px。
- 文字标签弱化，数值或判断加强。

#### 7.6.4 候选人卡片

结构：

- 姓名头像
- 姓名
- 岗位 / 能力 / 状态摘要

状态：

- 默认：弱白底、低透明边框。
- 选中：使用 `--accent-soft` 弱填充和 accent 混合边框。
- hover：轻微提高背景亮度。
- 空态：提示“当前筛选没有候选人，请清空搜索条件”。

#### 7.6.5 高层决策摘要

这是页面主组件，视觉权重必须最高。

组成：

- 匹配评分
- Executive Brief 标签
- 推进判断标题
- 解释性摘要
- 三段证据链
- 下一步动作按钮

规则：

- 匹配评分使用圆形或方形仪表，但不要过度拟物。
- 分数使用等宽字体，推荐 44px。
- 摘要标题可使用显示字体，但不得做成 Hero 式大字；优先表达“是否推进 / 风险 / 下一步动作”。
- 证据链必须包含“证据 / 风险 / 动作”，不能只写功能说明。

#### 7.6.6 面板

面板视觉：

```css
.panel {
  border: 1px solid var(--border);
  background:
    linear-gradient(180deg, rgba(255,255,255,0.06), transparent 130px),
    rgba(14, 18, 24, 0.82);
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.055);
}
```

规则：

- 圆角优先 0-4px，不使用 16px 以上大圆角。
- 不使用彩色左边框卡片。
- 不使用大阴影制造层级，优先使用边界、留白和背景亮度。

#### 7.6.7 按钮

基础要求：

- 最小高度：44px。
- 文字字距：0.02em。
- hover 有边界和背景变化。
- active 使用 `translateY(1px)`。
- disabled 必须降低透明度并禁止重复触发。

主按钮：

- 背景使用 accent 与黑色混合，不直接使用高亮纯色。
- 每屏强主按钮不超过 1 个。

次按钮：

- 使用透明弱白底。
- 只在 hover 时增强边界。

#### 7.6.8 输入框

规则：

- 高度不低于 44px。
- 背景使用弱白透明。
- placeholder 使用 `--faint`。
- focus 使用 accent outline，不能只靠颜色变化。

### 7.7 状态规范

#### 7.7.1 加载态

加载时必须同时做到：

- 按钮禁用。
- 设置 `aria-busy="true"`。
- 状态区域显示明确文案，例如“正在重新解析简历并刷新证据链...”。
- 完成后恢复按钮，并移除 `aria-busy`。

#### 7.7.2 空态

候选人列表为空时：

- 不显示空白区域。
- 给出可恢复建议，例如“请清空搜索条件”。
- 不要使用插画或夸张图标。

#### 7.7.3 错误态

错误态文案必须说明：

- 出了什么问题。
- 用户下一步可以做什么。
- 是否可以保留当前记录。

示例：

```text
解析服务暂时不可用，请稍后重试或导出当前已完成记录。
```

#### 7.7.4 成功态

成功态应克制，不弹出大面积庆祝效果。

推荐：

- 状态栏文字更新。
- 小型 toast。
- 候选人/证据链同步刷新。

### 7.8 动效规范

动效只用于反馈和状态变化，不用于装饰。

推荐参数：

- hover / active：140-180ms。
- 弹层显隐：160-220ms。
- 加载状态：可使用轻量 scanner 或状态文字变化。

规则：

- 必须支持 `prefers-reduced-motion: reduce`。
- 禁止大面积视差、复杂入场动画、循环霓虹光效。
- 不使用会影响阅读的抖动或自动滚动。

### 7.9 响应式规范

#### 7.9.1 断点

| 断点 | 行为 |
|---|---|
| 1180px 以下 | 左侧导航变为顶部横向，工作区改为单列 |
| 720px 以下 | 标题降到 42px，信号带单列，证据链单列 |

#### 7.9.2 移动端要求

- 按钮必须允许换行，不得挤压文字。
- 触控目标不小于 44px。
- 高层决策摘要仍然优先展示，不隐藏核心判断。
- 导航可以横向滚动，但不能遮挡内容。

### 7.10 可访问性规范

必须满足：

- 主区域使用 `aria-label` 或可见标题说明用途。
- 状态文本使用 `role="status"` 与 `aria-live="polite"`。
- 错误提示使用 `role="alert"`。
- 弹层使用 `role="dialog"` 或 `<dialog>`，并关联标题。
- icon-only 按钮必须有 `aria-label`。
- 键盘焦点必须可见。
- 禁止 `href="#"` 假链接。

焦点样式：

```css
button:focus-visible,
input:focus-visible {
  outline: 2px solid color-mix(in oklch, var(--accent), white 10%);
  outline-offset: 3px;
}
```

### 7.11 内容规范

#### 7.11.1 文案语气

文案要像招聘决策系统，不像营销广告。

推荐：

- “建议推进到技术追问”
- “项目规模待确认”
- “已写入岗位绑定记录”
- “权限边界已校验”

避免：

- “智能赋能招聘未来”
- “一键开启高效新时代”
- “10 倍提升招聘效率”
- “AI 全自动帮你搞定一切”

#### 7.11.2 数据原则

- 不编造外部指标。
- 没有真实数据时，用当前系统状态或短占位表达。
- 候选人分数、状态和动作必须能回到业务流程。

### 7.12 禁用清单

以下模式不得出现在本产品中：

- 默认紫色 / 靛蓝 accent。
- 紫蓝双色信任渐变。
- emoji 功能图标。
- 圆角卡片 + 彩色左边框。
- 暖米色、桃粉、棕橙色大背景。
- 无意义光团、blob、波浪背景。
- 假指标，例如“10x”“99.9%”“3 倍效率”。
- lorem ipsum、Feature One、占位式英文文案。
- 大量彩色图标装饰。
- 只换皮、不体现招聘决策链的普通后台布局。

### 7.13 交付检查清单

交付前逐项确认：

- [ ] 首屏是否能一眼看出当前候选人、推进判断、权限边界和下一动作？
- [ ] 顶部标题是否为任务型文案，且没有使用宣言式大字设计？
- [ ] 每屏 accent 是否控制在 2 到 3 个强可见位置？
- [ ] 面板是否使用金属细线和暗色材料，而不是普通白卡或彩边卡？
- [ ] 按钮是否有 hover、active、disabled、focus 状态？
- [ ] 输入、加载、空态、错误态是否都有可恢复反馈？
- [ ] 候选人、岗位、证据、风险和动作是否形成完整判断链？
- [ ] 小屏下是否没有横向溢出，按钮是否不挤压？
- [ ] 是否没有 emoji、默认紫色、假指标和模板化营销文案？
- [ ] 是否包含必要的 ARIA、键盘焦点和状态播报？

### 7.14 后续扩展建议

新增页面时应优先复用这套结构：

- 简历库：左侧筛选，中间候选人矩阵，右侧权限/流转上下文。
- 岗位模型：能力要求、风险模板、面试题模板三段证据链。
- 权限审计：用户角色、可见范围、操作记录和异常提醒。
- 通知中心：推荐提醒、主管反馈、超时任务和已读状态。

所有新增页面都必须保留“高层判断锚点 + 证据链 + 下一动作”的产品逻辑，不能退化成普通表格页。

---

## 8. DFX 设计要求

> DFX = Design For X。本文档中的 DFX 用于约束系统在安全、性能、可靠性、可维护性、可测试性、可观测性、可扩展性和隐私合规方面的工程质量。

### 8.1 DFX 总览

| 方向 | 目标 | 核心要求 |
|------|------|----------|
| Design for Security | 防越权、防泄露、防注入 | 后端鉴权为唯一可信边界;文件、搜索、导出均做权限校验 |
| Design for Performance | 常用操作低等待 | 列表秒级响应;AI 长任务异步化并显示状态 |
| Design for Reliability | 核心流程可恢复 | 上传、解析、推荐、通知支持失败提示、重试和幂等 |
| Design for Maintainability | 模块边界清晰 | 权限、简历、岗位、通知、审计等能力独立封装 |
| Design for Testability | 可自动化验证 | 权限矩阵、角色继承、推荐去重、AI 结果边界均有测试用例 |
| Design for Observability | 问题可定位 | 日志、指标、链路追踪、审计日志覆盖关键业务链路 |
| Design for Scalability | 数据与角色增长可承载 | 权限展开可缓存;异步任务可横向扩展 |
| Design for Privacy | 简历数据最小暴露 | 简历文件、解析结果、导出、日志均遵守最小必要原则 |

### 8.2 安全性

- 所有 API 必须先验证登录会话,再按 `user_can()` 校验资源与操作权限
- 前端不得把隐藏按钮、隐藏菜单作为权限边界;后端必须拒绝越权请求
- List 接口必须在数据库查询层应用权限谓词,不得先查全量数据再在前端过滤
- Resume / Position / UserDepartmentRole / Permission / RoleRelation 等敏感资源的 Create / Update / Delete 必须写审计日志
- 文件上传必须校验 MIME、扩展名、大小和解析安全;后端二次校验不得省略
- 搜索关键词、高亮内容、简历解析文本、JD 文本进入页面前必须进行 HTML 转义
- 导出面试题、导出列表等读取类能力必须复用 List / Get 权限校验结果
- W3 回调、登录态 Cookie / Token、接口 CSRF 防护、会话失效策略由后端统一处理

### 8.3 性能与容量

**交互性能目标**:

| 场景 | 目标 |
|------|------|
| 登录后加载默认主页 | P95 <= 2s,不含 W3 外部跳转耗时 |
| 列表页初次加载 | P95 <= 1s(1000 条以内) |
| 表格搜索 / 过滤 | 1000 条以内前端无明显卡顿 |
| 权限展开与过滤 | 单次 API P95 <= 200ms,复杂角色可使用缓存 |
| Toast / 选中 / 切换 tab | 100ms 内反馈 |
| AI 解析 / 分流 / 面试题生成 | 不承诺固定时长,必须展示可理解的进行中状态 |

**实现要求**:
- 列表接口分页或限制单次返回数量;即使当前验收按 <1000 条设计,接口也要预留分页能力
- 权限展开结果可按 `userId + roleVersion + departmentVersion` 缓存,UserDepartmentRole / Permission / RoleRelation 变更后立即失效
- 简历 PDF 解析、AI 结构化抽取、匹配计算、批量导入应进入后台任务,前端轮询或订阅任务状态
- 推荐分流计算应按渠道和在架岗位过滤后再计算,避免对无关岗位做无效评分
- 通知未读数使用轻量接口,不依赖拉取完整通知列表

### 8.4 可靠性与可恢复

- W3 认证超时按 Story 1.1 重试 1 次,仍失败则登录失败并记录日志
- PDF 上传成功但解析失败时,不得创建半完成业务数据;若已创建文件对象,应标记失败并可清理
- 批量导入按文件维度记录成功 / 失败,最终提示成功数和失败数;单个文件失败不得阻塞其他文件
- 推荐到目标部门必须幂等,以 6.2 推荐去重策略为准,重复点击不应创建重复简历副本
- Notification 创建失败不得回滚已完成的简历推荐主流程,但必须记录失败并支持后台补偿
- RoleRelation 递归展开必须有循环检测和最大深度保护,避免错误配置拖垮鉴权
- 后端所有写操作需返回明确错误码和用户可读错误信息,前端按错误类型展示可恢复操作

### 8.5 可维护性

- 权限模型、数据模型和业务服务应围绕本文资源边界拆分,避免把权限判断散落在各页面或控制器中
- `user_can()` 应作为统一鉴权入口,业务服务不得自行拼接不一致的权限逻辑
- UI 设计 token 应集中定义,组件不得硬编码大量临时颜色、阴影和字号
- Permission 白名单应集中维护,角色管理页和后端保存校验共用同一份业务定义
- AI 匹配算法的权重、阈值、标签规则应集中配置,避免在解析页和推荐页重复实现
- 审计日志字段、错误码、Toast 文案应有统一枚举或常量,避免多处漂移
- 数据库迁移必须可回滚;系统预置角色、Permission、RoleRelation 初始化脚本需要可重复执行

### 8.6 可测试性

**测试分层**:

| 层级 | 覆盖范围 |
|------|----------|
| 单元测试 | 权限展开、AttributeCondition 合并、匹配算法、推荐去重、字段校验 |
| 集成测试 | W3 登录回调、Resume 导入、PositionResume 创建、Notification 创建、RoleRelation 更新 |
| API 测试 | CRUD 权限矩阵、越权访问、数据范围过滤、重复绑定、循环角色关系 |
| E2E 测试 | 登录、游客页面权限、解析、推荐、简历库搜索、角色分配、通知跳转 |
| 可访问性测试 | 键盘操作、焦点态、对比度、减少动态效果 |

**必须覆盖的关键用例**:
- 多 UserDepartmentRole 取并集,并正确处理 system 部门
- 社招 / 校招负责人按 `attributeConditions.chan` 过滤简历
- HRD 绑定具体部门与绑定 system 部门时权限范围不同
- 角色禁用后下拉不可见,但已存在 UserDepartmentRole 仍生效
- 推荐同一候选人到同一部门不重复创建 Resume / DepartmentResume
- 解除所有业务角色后回退为游客状态

### 8.7 可观测性

- 后端日志使用结构化格式,至少包含 `requestId`、`userId`、`employeeId`、`roleSummary`、`resource`、`action`、`targetId`、`result`
- AI 解析 / 分流 / 面试题生成任务记录任务 ID、输入摘要、耗时、状态、失败原因
- 关键指标包括:登录成功率、W3 失败率、上传失败率、解析成功率、AI 任务耗时、推荐成功数、通知创建失败数、403 次数
- 所有审计日志必须可按操作人、资源、时间、操作类型检索
- 生产环境错误日志不得记录完整简历原文、完整 PDF 内容或超出排障需要的敏感个人信息

### 8.8 数据一致性与幂等

- Create Resume + Create DepartmentResume 必须处于同一事务;失败时不得留下无部门归属的有效简历
- Create Position + Create DepartmentPosition 必须处于同一事务
- Create Role + Permission + RoleRelation 必须处于同一事务
- UserDepartmentRole 同一 `(userId, departmentId, roleId)` 必须有唯一约束
- DepartmentResume 按"一份简历当前只归属一个部门"实现,同一 `resumeId` 仅允许一条有效记录
- 推荐去重查询与写入需要防并发重复,可使用事务锁、唯一键或幂等 key
- Notification 可按 `(to, resumeId, departmentId, positionId, by, timeWindow)` 做短窗口去重,避免重复点击造成刷屏

### 8.9 隐私与合规

- 简历 PDF、结构化简历、面试题、推荐通知均属于敏感招聘数据,只允许授权用户访问
- 日志、监控、错误上报不得输出完整简历正文、身份证号、手机号、邮箱等敏感字段
- 导出的 Markdown 面试题仅包含面试所需信息,不包含无关个人隐私字段
- 文件存储应支持访问控制和过期下载链接,不得暴露永久公开 URL
- 删除简历时按 Story 4.7 级联删除业务关联;审计日志仅保留必要快照,不保留完整简历正文
- 非生产环境测试数据必须脱敏,不得直接复制生产简历

### 8.10 可扩展性

- 新增渠道时优先扩展 `Resume.attributes.chan` 和 `Position.chan` 枚举,并复用 AttributeCondition,不得为每个渠道新增资源类型
- 新增角色能力时通过 Permission 白名单扩展,不得绕过 Role / Permission / RoleRelation 模型
- 新增 AI 模型或算法时通过 AI 服务适配层切换,页面与业务服务只依赖稳定的解析 / 匹配结果协议
- 新增通知渠道时复用 Notification 业务事件,可扩展为站内信、邮件、IM,但不改变推荐主流程
- 后续若简历规模超过单库查询能力,可引入搜索索引,但索引结果必须回到后端做权限过滤或索引内写入权限字段

---

## 9. 系统架构设计

### 9.1 架构原则

- **后端鉴权优先**:所有业务数据访问以后端权限判断为准,前端状态只提升体验
- **资源模型一致**:架构模块围绕 User / Department / Position / Resume / Role / Permission / Notification 等资源组织
- **同步主链路 + 异步重任务**:登录、查询、权限判断同步完成;PDF 解析、AI 抽取、批量导入、通知补偿异步处理
- **显式关系建模**:部门、岗位、简历、角色关系沿用 2.2 的关系实体,不在核心实体中重复维护关系字段
- **可解释 AI**:AI 输出必须被结构化保存或展示为可解释结果,业务判断不能只依赖黑盒文本

### 9.2 逻辑架构

```
Browser Web App
  ├─ Login / Session UI
  ├─ Resume Parse Workspace
  ├─ Resume Recommend Workspace
  ├─ Resume Library
  ├─ Department / Position Admin
  ├─ User / Role Admin
  └─ Notification Center

API / BFF Layer
  ├─ Session Middleware
  ├─ Permission Guard(user_can)
  ├─ Request Validation
  ├─ Error Mapping
  └─ Audit Hook

Domain Services
  ├─ Auth Service(W3 Adapter, session, single-device login)
  ├─ IAM Service(User, Role, Permission, UserDepartmentRole, RoleRelation)
  ├─ Department Service(Department, DepartmentPosition)
  ├─ Position Service(Position, JD, implicit tags)
  ├─ Resume Service(Resume, DepartmentResume, PositionResume)
  ├─ Matching Service(keyword score, implicit score, recommendation ranking)
  ├─ AI Service(PDF parsing, structure extraction, interview questions)
  ├─ Notification Service(Notification, unread count)
  └─ Audit Service(operation logs)

Infrastructure
  ├─ Relational Database(core business tables)
  ├─ Object Storage(PDF files, raw text references)
  ├─ Cache(permission expansion, session, unread count)
  ├─ Job Queue(PDF parse, batch import, AI tasks, notification compensation)
  └─ Observability(logs, metrics, traces, audit query)
```

### 9.3 模块职责

| 模块 | 职责 | 依赖 |
|------|------|------|
| Web App | 页面渲染、表单校验、状态管理、权限态展示、上传交互 | API / BFF |
| API / BFF | 聚合接口、会话校验、参数校验、错误映射、审计挂钩 | Domain Services |
| Auth Service | W3 登录、用户入库、会话生成、单设备登录控制 | W3、User、Session Cache |
| IAM Service | 角色、权限、角色继承、用户部门角色绑定、权限展开 | Role / Permission / UserDepartmentRole |
| Department Service | 部门主数据、部门与岗位 / 简历关系查询 | Department、DepartmentPosition、DepartmentResume |
| Position Service | JD、关键词、隐性标签、上下架、岗位归属 | Position、DepartmentPosition |
| Resume Service | 简历入库、归属、详情、删除、推荐副本、解析关系 | Resume、DepartmentResume、PositionResume |
| Matching Service | 匹配分计算、部门分流排序、阈值判断 | Resume、Position、Department |
| AI Service | PDF 文本抽取、结构化简历、面试题生成 | Object Storage、Job Queue、模型服务 |
| Notification Service | 推荐提醒、未读数、标记已读、通知跳转数据 | Notification、IAM |
| Audit Service | 操作留痕、审计查询、变更快照 | 所有写操作 |

### 9.4 数据存储边界

- **关系型数据库**:存储 2.1 / 2.2 定义的核心实体和关系实体,并承载事务一致性
- **对象存储**:存储原始 PDF、解析原文引用 `rawTextRef`,业务表只保存引用和结构化结果
- **缓存**:存储会话、权限展开结果、未读数等可重建数据;缓存失效不得影响权限正确性
- **任务队列**:存储异步任务状态和重试信息,任务结果最终写回业务表或返回前端展示
- **审计日志存储**:可与业务库分表或独立存储,但必须保证关键写操作成功后审计可查

### 9.5 权限校验链路

```
Request -> Session Middleware -> Permission Guard -> Domain Service -> Repository -> Response
```

校验要求:
- Session Middleware 负责确认用户已登录、会话未被后登录踢出
- Permission Guard 调用 IAM Service 展开 UserDepartmentRole + RoleRelation + Permission
- List 查询由 Permission Guard 生成可下推到数据库的访问谓词
- Get / Update / Delete 必须校验 target 是否落入任一允许范围
- Domain Service 不接收"前端传来的角色判断结果",只接收已认证的 `session.user`
- UserDepartmentRole / Permission / RoleRelation 变更后,相关用户权限缓存立即失效

### 9.6 关键业务链路

**登录链路**:
1. 用户点击 W3 登录
2. Auth Service 调用 W3 并校验在职状态
3. W3 返回 `id + name + employeeId`
4. 系统 Create / Update User
5. 新用户 Create 游客 UserDepartmentRole
6. 创建当前设备会话,并使同一 W3 id 的旧会话失效
7. 返回默认主页和角色摘要

**简历导入与解析链路**:
1. 前端上传 PDF 并选择渠道 / 目标部门
2. API 校验 Resume.Create + DepartmentResume.Create 权限和文件限制
3. 文件写入对象存储
4. 创建异步解析任务
5. AI Service 抽取结构化简历、关键词、traits、expBase
6. Resume Service 在事务中 Create Resume + DepartmentResume
7. 用户选择 JD 后触发匹配
8. Matching Service 计算得分,Resume Service Create PositionResume(kind=parsed)
9. 前端展示结果卡片和可解释明细

**智能分流与推荐链路**:
1. 用户选择简历并触发智能分流
2. API 校验 Resume.List / Get、Position.List、PositionResume.Create 等权限
3. Matching Service 获取权限范围内同渠道在架岗位
4. 对岗位计算匹配分,按部门聚合取最高分
5. 前端展示部门分流结果
6. 用户点击推荐到目标部门
7. Resume Service 按 `(normalizedName + targetDepartment)` 去重并创建或更新目标部门简历副本
8. Create / Update PositionResume(kind=recommended)
9. Notification Service 计算目标部门内有 Resume.List / Get 权限的收件人
10. Create N 条 Notification,自己给自己的通知默认 `read=true`
11. Audit Service 记录推荐行为

**角色变更链路**:
1. 超级管理员或授权管理角色发起 UserDepartmentRole / Permission / RoleRelation 变更
2. IAM Service 校验操作权限、重复绑定、RoleRelation 循环
3. 事务写入变更
4. Audit Service 记录变更前后值
5. 失效相关用户权限缓存
6. 后续 API 请求立即按新权限生效

### 9.7 API 分组建议

| 分组 | 示例能力 |
|------|----------|
| `/auth` | W3 登录回调、退出登录、当前会话、切换账号 |
| `/me` | 当前用户信息、角色摘要、权限范围摘要 |
| `/resumes` | 简历列表、详情、上传、删除、搜索、推荐副本 |
| `/positions` | 岗位列表、详情、新增、编辑、上下架、删除 |
| `/departments` | 部门列表、详情、新增、编辑、删除 |
| `/matching` | 解析匹配、智能分流、面试题生成 |
| `/users` | 用户列表、用户角色绑定、解除绑定 |
| `/roles` | 角色列表、创建、编辑、删除、启停、权限白名单 |
| `/notifications` | 未读数、通知列表、标记已读、跳转定位 |
| `/audit-logs` | 审计日志查询,默认仅超级管理员或审计授权角色可见 |
| `/jobs` | 异步任务状态查询、失败重试 |

API 设计要求:
- 所有写接口支持幂等或重复提交保护
- 错误响应统一包含 `code`、`message`、`requestId`
- List 接口统一支持分页、排序、搜索和权限范围过滤
- 文件上传接口与业务创建接口可拆分,但最终业务入库必须事务一致

### 9.8 异步任务设计

适合进入任务队列的场景:
- PDF 文本抽取与结构化解析
- 批量导入多份简历
- AI 生成面试题
- 大范围智能分流计算
- 通知失败补偿
- 审计日志异步归档

任务状态:

| 状态 | 含义 |
|------|------|
| `pending` | 已创建,等待执行 |
| `running` | 执行中 |
| `succeeded` | 成功,结果可读取 |
| `failed` | 失败,包含失败原因 |
| `cancelled` | 用户取消或系统终止 |

任务要求:
- 前端展示任务状态时必须保留用户输入上下文
- 失败任务允许用户重试,但重试不得重复创建业务数据
- 后台 Worker 必须以幂等方式处理任务,支持至少一次投递语义
- 长任务日志必须包含 `jobId` 和 `requestId`,便于排障

### 9.9 部署与环境

建议部署单元:
- Web 静态资源服务
- API / BFF 服务
- Worker 服务
- 关系型数据库
- 对象存储
- 缓存服务
- 任务队列
- 日志 / 指标 / 链路追踪服务

环境要求:
- 开发、测试、预发、生产环境隔离
- 生产环境不内置示例数据,与 5.5 保持一致
- 非生产环境可使用 W3 Mock 和 AI Mock,但必须标识环境,避免误连生产
- 配置项通过环境变量或配置中心管理,不得写入前端源码或提交到代码仓库
- 数据库迁移、预置角色初始化、Permission 白名单初始化应纳入发布流程

### 9.10 架构验收口径

- 任一业务 API 都能说明其所需 Resource / Action 以及数据范围来源
- 任一页面按钮显示状态都能映射到后端 Permission 检查结果
- 简历导入、解析、推荐、通知、角色变更均有审计日志和可追踪 requestId
- AI 服务失败不会破坏已提交的核心业务数据
- UserDepartmentRole / Permission / RoleRelation 修改后,下一次 API 调用立即体现新权限
- 新增角色、新增渠道、新增通知方式时,不需要重写现有核心业务链路
