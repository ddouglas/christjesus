import { readFileSync } from "fs";
import { resolve } from "path";

export interface TestAccount {
  email: string;
  userId: string;
}

export interface TestAccounts {
  baseURL: string;
  password: string;
  accounts: {
    recipient: TestAccount;
    donor: TestAccount;
    admin: TestAccount;
  };
}

const accountsPath = resolve(__dirname, "../test-accounts.json");

export const testAccounts: TestAccounts = JSON.parse(
  readFileSync(accountsPath, "utf-8")
);
