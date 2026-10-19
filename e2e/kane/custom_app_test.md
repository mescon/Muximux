---
mode: testing
url: http://localhost:4870/
max_steps: 50
---

# Custom app and sign-in

## Start setup and secure the dashboard
Type "muximux-e2e-setup-token" in the "Setup token" field and click "Let's Get Started".
Click "Create a password", type "samrivera" as the Username and "Muximux-Test-2026" as the Password and Confirm password, then click "Continue".
Verify the "What apps do you have?" step is shown.

## Add a custom app
Type "Home Router" in the "App name" field and "http://192.168.1.1" in the URL field next to it, then click "Add".
Click "Continue", then "Continue" again, then "Finish".
Verify the page shows "Your dashboard is ready with 1 app.".
Click "Launch Dashboard".
Verify the sidebar lists "Home Router".

## Sign out and back in
Click "Sign out" in the sidebar.
Verify a sign-in form is shown.
Type "samrivera" as the Username and "Muximux-Test-2026" as the Password, then sign in.
Verify the sidebar lists "Home Router".
