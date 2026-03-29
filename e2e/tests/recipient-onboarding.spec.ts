import { test, expect } from "@playwright/test";
import { loginAsRecipient } from "../helpers/auth";

test.describe("Recipient onboarding", () => {
  test("complete need submission flow", async ({ page }) => {
    // Step 0: Log in as the test recipient
    await loginAsRecipient(page);

    // Navigate to onboarding — e2e-reset ensures clean state (no userType, no draft needs)
    await page.goto("/onboarding");

    // How We Serve You — choose recipient path
    await page.waitForURL(/.*how-we-serve-you/);
    await page.getByRole("button", { name: "Share Your Need" }).click();

    // Welcome page
    await page.waitForURL(/.*\/welcome/);
    await page.getByRole("button", { name: "Let's get started" }).click();
    await page.waitForURL(/.*\/location/);
    // Step 3: Location — fill in a valid Charlotte address
    await expect(page).toHaveURL(/.*\/location/);

    // If there are existing saved addresses, select "new"
    const newAddressRadio = page.locator('input[name="address_selection"][value="new"]');
    if (await newAddressRadio.isVisible()) {
      await newAddressRadio.check();
    }

    await page.fill('input[name="address"]', "600 E 4th St");
    await page.fill('input[name="city"]', "Charlotte");
    await page.fill('input[name="state"]', "NC");
    await page.fill('input[name="zip_code"]', "28202");

    // Privacy display
    await page.locator('input[name="privacy_display"][value="zip"]').check();

    // Contact methods — check email
    await page.locator('input[name="contact_methods"][value="email"]').check();

    // Preferred contact time
    await page.selectOption('select[name="preferred_contact_time"]', "anytime");

    await page.getByRole("button", { name: "Continue" }).click();
    await page.waitForURL(/.*\/categories/);

    // Step 4: Categories — select a primary category
    await expect(page).toHaveURL(/.*\/categories/);

    // Select the first available primary category radio
    await page.locator('input[name="primary"]').first().check();

    await page.getByRole("button", { name: "Continue" }).click();
    await page.waitForURL(/.*\/story/);

    // Step 5: Story — fill amount and story fields
    await expect(page).toHaveURL(/.*\/story/);

    await page.fill("#amount", "1200");
    await page.fill(
      "#storyCurrent",
      "I recently lost my job and am struggling to cover rent this month."
    );
    await page.fill(
      "#storyNeed",
      "I need help with first month's rent at a new, more affordable apartment."
    );
    await page.fill(
      "#storyOutcome",
      "This support will give me stable housing while I start my new position next month."
    );

    await page.getByRole("button", { name: "Continue" }).click();
    await page.waitForURL(/.*\/documents/);

    // Step 6: Documents — skip for now
    await expect(page).toHaveURL(/.*\/documents/);

    // Check the "skip documents" checkbox and continue
    await page.locator("#skip-documents-checkbox").check();
    await page.getByRole("button", { name: "Continue" }).click();
    await page.waitForURL(/.*\/review/);

    // Step 7: Review — agree and submit
    await expect(page).toHaveURL(/.*\/review/);

    await page.locator("#agreeTerms").check();
    await page.locator("#agreeVerification").check();

    await page.getByRole("button", { name: "Submit Profile" }).click();
    await page.waitForURL(/.*\/confirmation/);

    // Step 8: Confirmation — verify we made it
    await expect(page).toHaveURL(/.*\/confirmation/);
    await expect(page.locator("text=Profile submitted")).toBeVisible();
  });
});
