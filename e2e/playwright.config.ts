import { defineConfig } from "@playwright/test";
import { readFileSync } from "fs";
import { resolve } from "path";

const accountsPath = resolve(__dirname, "test-accounts.json");
const accounts = JSON.parse(readFileSync(accountsPath, "utf-8"));

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: "html",
  use: {
    baseURL: accounts.baseURL || "http://localhost:8080",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: {
        browserName: "chromium",
        launchOptions: { slowMo: 500 },
      },
    },
  ],
});
