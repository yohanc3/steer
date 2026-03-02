import { useMutation } from "@tanstack/react-query";
import { useAPIFetch } from "./APIClient";
import { useUserStore } from "../store/useStore";

export default function useUser() {
  const { user: tryUser } = useUserStore();
  const { apiFetch } = useAPIFetch();

  const { mutate: onUserLogin } = useMutation({
    mutationFn: async (userID: string) => {
      console.log("Sending data to the server: ", userID);

      const res = await apiFetch(`user/${userID}`, {
        method: "POST",
        body: JSON.stringify({ ...tryUser }),
      });

      return res;
    },
  });

  return {
    onUserLogin,
  };
}
