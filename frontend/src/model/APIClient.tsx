const serverURL = import.meta.env.VITE_SERVER_BASE_URL;

export async function apiFetch(endpoint: string, options: RequestInit) {
  console.log("Sending req to: ", `${serverURL}/${endpoint}`);

  const response = await fetch(`${serverURL}/${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  const res = await response.json();

  return res;
}
