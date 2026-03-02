import { useAuth0 } from "@auth0/auth0-react";

const serverURL = import.meta.env.VITE_SERVER_BASE_URL;

export function useAPIFetch() {
  const { getAccessTokenSilently } = useAuth0();

  async function apiFetch(endpoint: string, options: RequestInit) {
    const accessToken = await getAccessTokenSilently(); 
    console.log("Sending req to: ", `${serverURL}/${endpoint} with access token: ${accessToken}`);

    const response = await fetch(`${serverURL}/${endpoint}`, {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${accessToken}`,
        ...options?.headers,
      },
      ...options,
    });

    const res = await response.json();

    return res;
  }

  return { apiFetch };
}
