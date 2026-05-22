import LoginButton from "./components/LoginButton";
import { useAuth0 } from "@auth0/auth0-react";
import Dashboard from "./components/Dashboard";
import { useEffect, useState } from "react";
import useUser from "./model/User";
import { useUserStore } from "./store/useStore";
import type { BaseUser } from "./types/types";

export default function App() {
    const { user, isAuthenticated, getAccessTokenSilently } = useAuth0();
    const { useOnUserLogin } = useUser();
    const { onUserLogin } = useOnUserLogin;
    const { setUser } = useUserStore();
    const [isInitialized, setInitialized] = useState<boolean>(false);

    // Push user to db in the backend only if logged in
    useEffect(() => {
        if (!isAuthenticated || isInitialized || !user || !user.sub || !user.email || !user.name)
            return;

        async function initializeUser() {
            try {
                const userData: BaseUser = {
                    ID: user!.sub!,
                    email: user!.email!,
                    name: user!.name!,
                    picture: user?.picture || null,
                };

                const onUserLoginData = await onUserLogin(userData);

                setUser({ ...userData, ...onUserLoginData });
                setInitialized(true);
            } catch (e) {
                console.error("Error when initializing user: ", e);
            }
        }

        initializeUser();
    }, [user, onUserLogin, setUser, getAccessTokenSilently, isInitialized, user]);

    if (!isAuthenticated) {
        return <LoginButton />;
    }

    if (!isInitialized) {
        return <div> Setting up your account... </div>;
    }

    return <main>{isAuthenticated && <Dashboard />}</main>;
}
