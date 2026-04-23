export const minPasswordCharacters = 8;
export const maxPasswordBytes = 72;

export function getPasswordByteLength(value: string) {
  return new TextEncoder().encode(value).length;
}

export function getPasswordCharacterCount(value: string) {
  return Array.from(value).length;
}
