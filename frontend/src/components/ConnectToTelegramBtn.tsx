import { useEffect, useState } from "react";
import useUser from "../model/User";
import { useUserStore } from "../store/useStore";

export default function ConnectToTelegramBtn() {
    const { user } = useUserStore();
    const { getTelegramCode } = useUser();
    const { data, refetch } = getTelegramCode;
    const [codeExpirationSecondsLeft, setCodeExpirationSecondsLeft] = useState<number>(0);
    console.log("expiration seconds left: ", codeExpirationSecondsLeft);

    useEffect(() => {
        if (!user.ID || !data?.expires_at) {
            return;
        }
        const updateSecondsLeftIntervalID = setInterval(() => {
            setCodeExpirationSecondsLeft(() => {
                const secondsLeft = Math.max(
                    0,
                    new Date(data.expires_at - Date.now()).getSeconds()
                );
                console.log("updating seconds before expiration to ", secondsLeft);

                if (secondsLeft == 0) {
                    refetch();
                }

                return secondsLeft;
            });
        }, 1000);

        return () => {
            clearInterval(updateSecondsLeftIntervalID);
        };
    }, [data, data?.code, data?.expires_at, refetch, user.ID]);

    return (
        <>
            {user.isConnectedToTelegram ? (
                <div>
                    <div>Seconds before refetching new code {codeExpirationSecondsLeft}</div>
                    <button
                        onClick={async () => {
                            const token = data?.code;
                            window.open(`https://t.me/OfficialSteerBot?start=${token}`, "_blank");
                        }}
                    >
                        Connect to Telegram
                    </button>{" "}
                </div>
            ) : (
                <div>connected to telegram</div>
            )}
        </>
    );
}
