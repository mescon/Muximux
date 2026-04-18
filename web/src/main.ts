import './app.css';
import { mount } from 'svelte';
import { getLocale } from '$lib/paraglide/runtime.js';
import { applyLocaleToDocument } from '$lib/localeStore';
import App from './App.svelte';

// Set lang/dir on <html> before first render (read from localStorage or baseLocale)
applyLocaleToDocument(getLocale());

const app = mount(App, { target: document.getElementById('app')! });

export default app;
