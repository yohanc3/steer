import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "./APIClient";

export default function useUser(email: string) {
  const { mutate: onUserLogin } = useMutation({
    mutationFn: async () => {
      console.log("Sending data to the server");

      const res = await apiFetch(`user/${email}`, {
        method: "POST",
      });

      return res;
    },
  });

  return {
    onUserLogin,
  };
}
