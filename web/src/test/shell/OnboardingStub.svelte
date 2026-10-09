<script lang="ts">
  let { oncomplete }: { oncomplete: (d: unknown) => void } = $props();
  // A test can set globalThis.__onboardingDetail to control what the wizard
  // hands back; the default is a first-run setup with no picks.
  function detail(): unknown {
    const custom = (globalThis as Record<string, unknown>).__onboardingDetail;
    return custom ?? {
      apps: [], navigation: {}, groups: [], theme: { family: 'default', variant: 'dark' },
      language: 'en', setup: { method: 'none' },
    };
  }
</script>
<button
  data-testid="onboarding-done"
  onclick={() => oncomplete(detail())}
>done</button>
