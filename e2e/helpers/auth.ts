import { type Page } from "@playwright/test";
import { testAccounts, type TestAccount } from "./accounts";

/**
 * Log in through the Auth0 Universal Login page.
 *
 * Navigates to /login, submits through to Auth0, fills credentials,
 * and waits for the redirect back to the app.
 */
export async function loginAs(page: Page, account: TestAccount) {
  // Navigate to /login — the app immediately redirects to Auth0
  await page.goto("/login");
  await page.waitForURL(/.*auth0\.com.*/);

  // Fill Auth0 Universal Login form
  await page.fill('input[name="username"], input[name="email"]', account.email);
  await page.fill('input[name="password"]', testAccounts.password);
  await page.getByRole("button", { name: "Continue", exact: true }).click();

  // Auth0 may show a consent/authorize screen
  const acceptButton = page.getByRole("button", { name: "Accept" });
  if (await acceptButton.isVisible({ timeout: 3000 }).catch(() => false)) {
    await acceptButton.click();
  }

  // Wait for redirect back to the app
  await page.waitForURL(/.*localhost.*/);
}

export async function loginAsRecipient(page: Page) {
  await loginAs(page, testAccounts.accounts.recipient);
}

export async function loginAsDonor(page: Page) {
  await loginAs(page, testAccounts.accounts.donor);
}

export async function loginAsAdmin(page: Page) {
  await loginAs(page, testAccounts.accounts.admin);
}
