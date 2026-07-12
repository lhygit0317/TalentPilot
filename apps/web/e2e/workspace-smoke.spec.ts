import { expect, test } from "@playwright/test";
import { installApiMocks } from "./api-fixtures";

test.beforeEach(async ({ page }) => {
  await installApiMocks(page);
});

test("authenticated shell renders permission-scoped navigation", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByText("A00001")).toBeVisible();
  await expect(page.getByRole("link", { name: "简历解析" })).toBeVisible();
  await expect(page.getByRole("link", { name: "简历推荐" })).toBeVisible();
  await expect(page.getByRole("link", { name: "用户管理" })).toBeVisible();
  await expect(page.getByRole("link", { name: "角色管理" })).toBeVisible();
  await expect(page.getByRole("button", { name: "推荐提醒" })).toBeVisible();
});

test("parses a resume and generates interview questions", async ({ page }) => {
  await page.goto("/#resume-parse");

  await page.getByRole("button", { name: /张三/ }).click();
  await page.getByRole("button", { name: "开始解析" }).click();
  await expect(page.getByText("建议进入面试", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "生成面试题" }).click();
  await page.getByRole("tab", { name: "主管" }).click();
  await expect(page.getByText(/为什么选择算力训练平台部/)).toBeVisible();
});

test("sends a recommendation and consumes notification read state", async ({ page }) => {
  await page.goto("/#resume-recommend");

  await page.getByRole("button", { name: /张三/ }).click();
  await page.getByRole("button", { name: "智能分流" }).click();
  await expect(page.getByText("最佳去向")).toBeVisible();
  await page.getByRole("button", { name: "推荐到" }).click();
  await expect(page.getByRole("status")).toContainText("已推荐到「智算调度部」");

  await page.getByRole("button", { name: "推荐提醒" }).click();
  await page.getByRole("button", { name: /张三 被推荐到/ }).click();
  await expect(page.getByRole("heading", { name: "简历库" })).toBeVisible();
  await expect(page.getByText("有 1 份简历被推荐到你可查看的部门")).toBeVisible();
  await expect(page.getByRole("button", { name: "推荐提醒" })).toContainText("1");

  await page.getByRole("button", { name: "推荐提醒" }).click();
  await expect(page.getByText("暂无新的推荐提醒")).toBeVisible();
  await expect(page.getByRole("button", { name: /张三 被推荐到/ })).toHaveCount(0);
  await page.getByRole("button", { name: "全部已读" }).click();
  await expect(page.getByRole("status")).toContainText("已全部标记为已读");
  await expect(page.getByRole("button", { name: "推荐提醒" })).toHaveText("推荐提醒");
});
