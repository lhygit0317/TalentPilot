import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { findViolations } from "./check-business-ui-elements.mjs";

const tempRoots = [];

async function makeRoot() {
  const root = await mkdtemp(join(tmpdir(), "talentpilot-ui-check-"));
  tempRoots.push(root);
  return root;
}

async function write(root, file, source) {
  const path = join(root, file);
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, source, "utf8");
}

describe("check-business-ui-elements", () => {
  afterEach(async () => {
    await Promise.all(tempRoots.splice(0).map((root) => rm(root, { force: true, recursive: true })));
  });

  it("reports raw interactive elements in business TSX files", async () => {
    const root = await makeRoot();
    await write(root, "src/resume-library/BadPage.tsx", "export function BadPage() { return <button>Bad</button>; }");

    const violations = await findViolations(root);

    expect(violations).toEqual([
      expect.objectContaining({
        element: "button",
        file: expect.stringContaining("src/resume-library/BadPage.tsx"),
      }),
    ]);
  });

  it("allows UI wrappers and test files to render raw elements", async () => {
    const root = await makeRoot();
    await write(root, "src/components/ui/button.tsx", "export function Button() { return <button>OK</button>; }");
    await write(root, "src/users/UsersPage.test.tsx", "export function Fixture() { return <input />; }");
    await write(root, "src/users/UsersPage.tsx", "export function UsersPage() { return <Button />; }");

    await expect(findViolations(root)).resolves.toEqual([]);
  });
});
