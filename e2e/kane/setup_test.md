---
mode: testing
url: http://localhost:4870/
max_steps: 50
---

# Onboarding wizard

## Start setup and secure the dashboard
Type "muximux-e2e-setup-token" in the "Setup token" field and click "Let's Get Started".
Click "Create a password", type "alexmorgan" as the Username and "Muximux-Test-2026" as the Password and Confirm password, then click "Continue".
Verify the "What apps do you have?" step is shown.

## Pick an app, layout and theme
In the App Catalog, click "Jellyfin", then click "Continue".
On the "Choose Your Navigation Style" step, click "Continue".
On the "Choose Your Theme" step, click the "Nord" theme, then click "Finish".
Verify the page shows "You're All Set!", "Your dashboard is ready with 1 app." and the theme "Nord" in the setup summary.

## Open the dashboard
Click "Launch Dashboard".
Verify the sidebar lists "Jellyfin" under "MEDIA" and the page shows "Select an application to get started".
