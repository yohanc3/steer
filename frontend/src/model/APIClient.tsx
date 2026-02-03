import { useAccessTokenStore } from "../store/useStore";

const serverURL = import.meta.env.VITE_SERVER_BASE_URL;

export async function apiFetch(endpoint: string, options: RequestInit) {
  const accessToken = useAccessTokenStore.getState().accessToken
  console.log("Sending req to: ", `${serverURL}/${endpoint} with access token: ${accessToken}`);
  

  const response = await fetch(`${serverURL}/${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${accessToken}`,
      ...options?.headers,
    },
  });

  const res = await response.json();

  return res;
}
