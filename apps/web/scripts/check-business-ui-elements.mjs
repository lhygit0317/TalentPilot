import { readdir, readFile } from "node:fs/promises";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";

const disallowedElements = new Set(["button", "input", "select", "textarea", "dialog", "form", "table"]);
const ignoredDirectories = new Set(["dist", "node_modules"]);

function normalize(path) {
  return path.split(sep).join("/");
}

function isIgnored(file) {
  const normalized = normalize(file);
  const segments = normalized.split("/");

  return (
    normalized.includes("/components/ui/") ||
    segments.includes("test") ||
    segments.includes("tests") ||
    segments.includes("test-setup") ||
    segments.includes("setup") ||
    normalized.endsWith(".test.tsx") ||
    normalized.endsWith(".test.ts") ||
    normalized.endsWith(".spec.tsx") ||
    normalized.endsWith(".spec.ts")
  );
}

async function collectTsxFiles(dir, files = []) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch (error) {
    if (error?.code === "ENOENT") {
      return files;
    }
    throw error;
  }

  for (const entry of entries) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (!ignoredDirectories.has(entry.name)) {
        await collectTsxFiles(path, files);
      }
      continue;
    }

    if (entry.isFile() && path.endsWith(".tsx") && !isIgnored(path)) {
      files.push(path);
    }
  }

  return files;
}

function jsxTagName(node) {
  const tag = node.tagName;
  return ts.isIdentifier(tag) ? tag.text : "";
}

export async function findViolations(root = process.cwd()) {
  const srcRoot = join(root, "src");
  const files = await collectTsxFiles(srcRoot);
  const violations = [];

  for (const file of files) {
    const source = await readFile(file, "utf8");
    const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

    function visit(node) {
      if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
        const element = jsxTagName(node);
        if (disallowedElements.has(element)) {
          const { line, character } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
          violations.push({
            column: character + 1,
            element,
            file: relative(root, file),
            line: line + 1,
          });
        }
      }

      ts.forEachChild(node, visit);
    }

    visit(sourceFile);
  }

  return violations;
}

export function formatViolations(violations) {
  return violations
    .map(
      (violation) =>
        `${violation.file}:${violation.line}:${violation.column} raw <${violation.element}> is not allowed in business pages`,
    )
    .join("\n");
}

async function main() {
  const root = join(dirname(fileURLToPath(import.meta.url)), "..");
  const violations = await findViolations(root);
  if (violations.length > 0) {
    console.error(formatViolations(violations));
    process.exitCode = 1;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
