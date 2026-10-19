# End-to-end (e2e) tests for web

Browser tests written in plain English for [Kane CLI](https://www.testmuai.com/kane-cli/).

| Test | What it covers |
|---|---|
| `setup_test.md` | The onboarding wizard: password auth, an app from the catalog, a theme and the dashboard |
| `custom_app_test.md` | Onboarding with a custom app, then signing out and back in |

## Run

Start a fresh instance on port 4870 with a known setup token:

```bash
docker create --name muximux-e2e -p 4870:8080 ghcr.io/mescon/muximux:latest
echo muximux-e2e-setup-token > /tmp/muximux-setup-token
docker cp /tmp/muximux-setup-token muximux-e2e:/app/data/.setup-token
docker start muximux-e2e
npm install -g @testmuai/kane-cli
kane-cli login
kane-cli testmd run e2e/kane/setup_test.md
kane-cli testmd run e2e/kane/custom_app_test.md
```

Both tests start from the onboarding wizard, so recreate the container before each run.
