import { useUserStore } from "../store/useStore";
import ConnectToTelegramBtn from "./ConnectToTelegramBtn";
import ConnectBank from "./ConnectBank.tsx";
import LogoutButton from "./LogoutButton";

export default function Dashboard() {
    const { user } = useUserStore();

    return (
        <div>
            <LogoutButton />
            <div>
                {Object.entries(user).map(([key, value]) => (
                    <p key={key}>
                        {key}: {value}
                    </p>
                ))}
            </div>
            <p>user name: {user.email} </p>
            <p>user connected to telegram: {user.isConnectedToTelegram} </p>
            <ConnectToTelegramBtn />

            <ConnectBank />
        </div>
    );
}
