import { useMutation, useQuery } from "@tanstack/react-query";
import { useAPIFetch } from "./APIClient";
import { extraUserDataSchema, connectionCodeSchema, type connectionCode, type BaseUser } from "../types/types";
import { useUserStore } from "@/store/useStore";

export default function useUser() {
    const { apiFetch } = useAPIFetch();
    const {user: userStore} = useUserStore()

    const {
        isSuccess: isUserLoginSuccess,
        mutateAsync: onUserLogin,
        error: onUserLoginError,
    } = useMutation({
        mutationFn: async (user: BaseUser) => {
            console.log("Sending data to the server: ", user);

            const userData = await apiFetch(extraUserDataSchema, "user", {
                method: "POST",
                body: JSON.stringify({ ...user }),
            });

            return userData;
        },
        onSuccess: (data) => {
            return data;
        },
    });

    async function fetchTelegramCode(userID: string): Promise<connectionCode> {
        console.log("running telegram fetch with user id: ", userID);

        try {
            const code = await apiFetch(
                connectionCodeSchema,
                `user/${userID}/telegram/connection-code`,
                {
                    method: "GET",
                }
            );

            if (code instanceof Error) {
                throw new Error("Error when fetching code." + code);
            }
            console.log("result:", code);
            return code;
        } catch (e) {
            throw new Error("Expected JSON of type`connectionCodeJSON`. Error: " + e);
        }
    }

    const getTelegramCode = useQuery({
        queryFn: () => fetchTelegramCode(userStore.ID),
        queryKey: ["getTelegramCode", userStore.ID],
        enabled: !!userStore.ID
    });

    return {
        useOnUserLogin: {
            onUserLogin,
            isUserLoginSuccess,
            onUserLoginError,
        },
        getTelegramCode,
    };
}
