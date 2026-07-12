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
