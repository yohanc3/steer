import "./App.css";
import LoginButton from "./components/LoginButton";
import { useAuth0 } from "@auth0/auth0-react";
import Dashboard from "./components/Dashboard";
import { useEffect, useState } from "react";
import useUser from "./model/User";
import { useAccessTokenStore, useUserStore, type BaseUser } from "./store/useStore";

export default function App() {
  const { user, isAuthenticated, getAccessTokenSilently } = useAuth0();
  const { onUserLogin } = useUser();
  const { setAccessToken, accessToken } = useAccessTokenStore();
  const { setUser } = useUserStore();
  const [isInitialized, setInitialized] = useState<boolean>(false);
  const [isTokenReady, setIsTokenReady] = useState<boolean>(false);

  // Push user to db in the backend only if not logged in
  useEffect(() => {
    if (!isAuthenticated || isInitialized || isTokenReady || !user || !user.sub || !user.email || !user.name)
      return;

    async function initializeUser() {
      try {
        const userData: BaseUser = {
          userID: user!.sub!,
          email: user!.email!,
          name: user!.name!,
          picture: user?.picture || null,
        };
        setUser({ ...userData });

        const accessToken = await getAccessTokenSilently();
        setAccessToken(accessToken);
        setIsTokenReady(true)

        onUserLogin(userData.userID);
        setInitialized(true);
      } catch (e) {
        console.error("Error when initializing user: ", e);
      }
    }

    initializeUser();
  }, [user, onUserLogin, setUser, getAccessTokenSilently, isInitialized, setAccessToken, accessToken, user]);

  if (!isAuthenticated){
    return <LoginButton /> 
  }

  if (!isInitialized || !isTokenReady) {
    return <div> Setting up your account... </div>;
  }
  return (
    <main>
      {isAuthenticated && isTokenReady && <Dashboard />}
    </main>
  );
}
