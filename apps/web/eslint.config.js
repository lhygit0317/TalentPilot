import tseslint from "typescript-eslint";

export default [
  {
    ignores: ["dist/**", "node_modules/**"],
  },
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{js,ts,tsx}"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
      globals: {
        document: "readonly",
        HTMLElement: "readonly",
        React: "readonly",
      },
      parserOptions: {
        ecmaFeatures: {
          jsx: true,
        },
      },
    },
    rules: {
      "no-undef": "error",
    },
  },
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "no-undef": "off",
    },
  },
];
