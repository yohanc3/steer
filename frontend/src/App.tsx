import "./App.css";
import LoginButton from "./components/LoginButton";
import { useAuth0 } from "@auth0/auth0-react";
import Dashboard from "./components/Dashboard";
import { useUserStore } from "./store/userStore";
import { useEffect } from "react";

function App() {
  const { user: auth0User, isLoading, isAuthenticated } = useAuth0();
  const setUser = useUserStore((state) => state.setUser);
  const clearUser = useUserStore((state) => state.clearUser);

  useEffect(() => {
    if (isAuthenticated && auth0User) {
      setUser({
        email: auth0User.email!,
        name: auth0User.name,
        picture: auth0User.picture,
      });
    } else {
      clearUser();
    }
  }, [isAuthenticated, auth0User, setUser, clearUser]);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (!isAuthenticated || !auth0User) {
    return <LoginButton />;
  }

  return (
    <main>
      <Dashboard />
    </main>
  );
}

export default App;
