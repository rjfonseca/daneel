import { act, render } from '@testing-library/react';

// Let deferred component effects settle before assertions or test cleanup.
export async function renderAsync(...args: Parameters<typeof render>) {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(...args);
  });
  return result;
}
