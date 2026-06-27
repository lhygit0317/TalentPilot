import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: "oklch(10% 0.018 245)",
        shell: "oklch(13% 0.018 245)",
        fg: "oklch(93% 0.01 240)",
        muted: "oklch(66% 0.018 240)",
        accent: "oklch(73% 0.13 190)",
      },
      fontFamily: {
        body: [
          "-apple-system",
          "BlinkMacSystemFont",
          "SF Pro Text",
          "PingFang SC",
          "Microsoft YaHei",
          "sans-serif",
        ],
      },
    },
  },
  plugins: [],
} satisfies Config;
