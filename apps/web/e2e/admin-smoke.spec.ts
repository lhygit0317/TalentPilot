import { expect, test } from "@playwright/test";
import { installApiMocks } from "./api-fixtures";

test.beforeEach(async ({ page }) => {
  await installApiMocks(page);
});

test("assigns and removes a user role binding", async ({ page }) => {
  await page.goto("/#users");

  const row = page.getByRole("row", { name: /张敏/ });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "分配角色" }).click();
  const dialog = page.getByRole("dialog", { name: "角色分配" });
  await dialog.getByLabel("角色").selectOption("__role_manager__");
  await dialog.getByLabel("部门").selectOption("dept_b");
  await page.getByRole("button", { name: "加入待分配" }).click();
  await page.getByRole("button", { name: "保存分配" }).click();

  await expect(page.getByRole("status")).toContainText("已为 张敏 分配 1 个角色绑定");
  await expect(page.getByRole("row", { name: /张敏/ })).toContainText("主管");

  await page.getByRole("row", { name: /张敏/ }).getByRole("button", { name: "解除" }).click();

  await expect(page.getByRole("status")).toContainText("已解除 HRBP");
  await expect(page.getByRole("row", { name: /张敏/ })).not.toContainText("HRBP");
});

test("creates a custom role and shows invalid relation errors", async ({ page }) => {
  await page.goto("/#roles");

  await page.getByRole("button", { name: "新建角色" }).click();
  let dialog = page.getByRole("dialog", { name: "角色定义" });
  await dialog.getByLabel("角色名称").fill("高级评审者");
  await dialog.getByLabel("角色描述").fill("查看社招简历");
  await dialog.getByLabel("Resume List").check();
  await dialog.getByLabel("社招").check();
  await dialog.getByRole("button", { name: "保存角色" }).click();
  await expect(page.getByRole("status")).toContainText("已创建角色");
  await expect(page.getByRole("row", { name: /高级评审者/ })).toBeVisible();

  await page.getByRole("button", { name: "新建角色" }).click();
  dialog = page.getByRole("dialog", { name: "角色定义" });
  await dialog.getByLabel("角色名称").fill("循环角色");
  await dialog.getByLabel("包含 HRBP").check();
  await dialog.getByRole("button", { name: "保存角色" }).click();
  await expect(page.getByRole("alert")).toContainText("角色包含关系不能形成循环");
});
