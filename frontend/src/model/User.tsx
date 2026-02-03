import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "./APIClient";
import { useAccessTokenStore, useUserStore} from "../store/useStore";

export default function useUser() {

  const {user: tryUser} = useUserStore()
  const {accessToken} = useAccessTokenStore()

  console.log("user from user store: ", tryUser)
  console.log("access token: ", accessToken)

  const { mutate: onUserLogin } = useMutation({
    mutationFn: async (userID: string) => {
      console.log("Sending data to the server: ", userID);

     const res = await apiFetch(`user/${userID}`, {
        method: "POST",
        body: JSON.stringify({...tryUser}),
        headers: {
          "Authorization": `Bearer ${accessToken}`
        }
      });

      return res;
    },
  });

  return {
    onUserLogin,
  };
}

// NEXT UP: clean to not need user stuff. ask only for what's needed inside every mutation. also
// clean up app.tsx, to only load stuff when users are actually authenticated.
