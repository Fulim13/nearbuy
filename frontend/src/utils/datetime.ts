// <input type="datetime-local"> speaks local "YYYY-MM-DDTHH:mm"; the voucher
// API wants RFC3339. These two convert between them.
export const toLocalInput = (date: Date) => {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
};

export const localInputToRfc3339 = (value: string) =>
  new Date(value).toISOString().replace(/\.\d{3}Z$/, 'Z');
