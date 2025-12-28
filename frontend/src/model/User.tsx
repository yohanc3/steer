import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "./APIClient";
import type { BaseUser } from "../store/userStore";

export default function useUser(user: BaseUser) {
  const { mutate: onUserLogin } = useMutation({
    mutationFn: async () => {
      console.log("Sending data to the server: ", user);

     const res = await apiFetch(`user/${user.email}`, {
        method: "POST",
        body: JSON.stringify(user)
      });

      return res;
    },
  });

  return {
    onUserLogin,
  };
}
