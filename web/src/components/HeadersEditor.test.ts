import { describe, it, expect } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import HeadersEditor from './HeadersEditor.svelte';

function setup(initial: Record<string, string> = {}, onChange: (v: Record<string, string>) => void = () => {}) {
  return render(HeadersEditor, { props: { value: initial, onChange } });
}

describe('HeadersEditor', () => {
  it('renders one row per entry plus an empty add button', () => {
    const { getAllByPlaceholderText, getByRole } = setup({ Authorization: 'Bearer abc', 'X-Tok': '1' });
    expect(getAllByPlaceholderText(/header name/i).length).toBe(2);
    expect(getByRole('button', { name: /add header/i })).toBeTruthy();
  });

  it('renders zero rows when value is empty', () => {
    const { queryAllByPlaceholderText, getByRole } = setup({});
    expect(queryAllByPlaceholderText(/header name/i).length).toBe(0);
    expect(getByRole('button', { name: /add header/i })).toBeTruthy();
  });

  it('clicking add inserts a new empty row', async () => {
    const { getByRole, getAllByPlaceholderText } = setup({});
    await fireEvent.click(getByRole('button', { name: /add header/i }));
    await waitFor(() => {
      expect(getAllByPlaceholderText(/header name/i).length).toBe(1);
    });
  });

  it('typing a key + value calls onChange with the flushed map', async () => {
    let captured: Record<string, string> = {};
    const { getByRole, getAllByPlaceholderText } = setup({}, (v) => { captured = v; });
    await fireEvent.click(getByRole('button', { name: /add header/i }));
    const keyInput = getAllByPlaceholderText(/header name/i)[0] as HTMLInputElement;
    const valueInput = getAllByPlaceholderText(/header value/i)[0] as HTMLInputElement;
    await fireEvent.input(keyInput, { target: { value: 'Authorization' } });
    await fireEvent.input(valueInput, { target: { value: 'Bearer abc' } });
    await waitFor(() => {
      expect(captured).toEqual({ Authorization: 'Bearer abc' });
    });
  });

  it('removing a row removes it from onChange output', async () => {
    let captured: Record<string, string> = { Authorization: 'Bearer abc' };
    const { getAllByRole } = setup({ Authorization: 'Bearer abc' }, (v) => { captured = v; });
    const removeBtn = getAllByRole('button', { name: /remove header/i })[0];
    await fireEvent.click(removeBtn);
    await waitFor(() => {
      expect(captured).toEqual({});
    });
  });

  it('empty keys are dropped on flush', async () => {
    let captured: Record<string, string> = {};
    const { getByRole, getAllByPlaceholderText } = setup({}, (v) => { captured = v; });
    await fireEvent.click(getByRole('button', { name: /add header/i }));
    const valueInput = getAllByPlaceholderText(/header value/i)[0] as HTMLInputElement;
    await fireEvent.input(valueInput, { target: { value: 'orphan-value' } });
    await waitFor(() => {
      expect(captured).toEqual({});
    });
  });

  it('duplicate keys: last row wins', async () => {
    let captured: Record<string, string> = {};
    const { getByRole, getAllByPlaceholderText } = setup({}, (v) => { captured = v; });
    await fireEvent.click(getByRole('button', { name: /add header/i }));
    await fireEvent.click(getByRole('button', { name: /add header/i }));
    const [k1, k2] = getAllByPlaceholderText(/header name/i) as HTMLInputElement[];
    const [v1, v2] = getAllByPlaceholderText(/header value/i) as HTMLInputElement[];
    await fireEvent.input(k1, { target: { value: 'X-Tok' } });
    await fireEvent.input(v1, { target: { value: 'first' } });
    await fireEvent.input(k2, { target: { value: 'X-Tok' } });
    await fireEvent.input(v2, { target: { value: 'second' } });
    await waitFor(() => {
      expect(captured).toEqual({ 'X-Tok': 'second' });
    });
  });
});
