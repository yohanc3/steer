import { useAuth0 } from "@auth0/auth0-react";
import * as z from "zod";

const serverURL = import.meta.env.VITE_SERVER_BASE_URL;

export function useAPIFetch() {
  const { getAccessTokenSilently } = useAuth0();

  async function apiFetch<T extends z.ZodType>(fetchedDataExpectedSchema: T, endpoint: string,  options?: RequestInit): Promise<z.output<T>> {
    let accessToken;
    try {
      accessToken = await getAccessTokenSilently();
    } catch (e) {
      throw new Error("error when getting access token silently: " + e) 
    }

    console.log(
      "Sending req to: ",
      `${serverURL}/api/${endpoint} with access token: ${accessToken}`
    );

    const response = await fetch(`${serverURL}/api/${endpoint}`, {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${accessToken}`,
        ...options?.headers,
      },
      ...options,
    });

    const responseJSON = await response.json();

    const parsedResult = fetchedDataExpectedSchema.safeParse(responseJSON) 

    if (parsedResult.error) {
         throw new Error("api result type is different than expected.", parsedResult.error) 
    }

    return parsedResult.data;
  }

  return { apiFetch };
}
