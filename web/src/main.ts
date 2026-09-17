import './app.css';
import { mount } from 'svelte';
import { getLocale } from '$lib/paraglide/runtime.js';
import { applyLocaleToDocument } from '$lib/localeStore';
import { rescueMisroutedFrame } from '$lib/frameRescue';
import App from './App.svelte';

// A proxied app frame that reloaded or navigated without its /proxy/<slug>
// prefix lands here. Send it back to the proxy instead of nesting the shell.
const rescued = rescueMisroutedFrame();

// Set lang/dir on <html> before first render (read from localStorage or baseLocale)
if (!rescued) applyLocaleToDocument(getLocale());

const app = rescued ? undefined : mount(App, { target: document.getElementById('app')! });

export default app;
