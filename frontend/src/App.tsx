import "./App.css";
import LoginButton from "./components/LoginButton";
import { useAuth0 } from "@auth0/auth0-react";
import Dashboard from "./components/Dashboard";
import { type BaseUser } from "./store/userStore";

function App() {
  const { user: auth0User, isLoading, isAuthenticated } = useAuth0();
  console.log("auth0 user: ", auth0User);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (
    !isAuthenticated ||
    !auth0User ||
    !auth0User.email ||
    !auth0User.name ||
    !auth0User.picture
  ) {
    return <LoginButton />;
  }

  const user: BaseUser = {
    email: auth0User.email,
    name: auth0User.name,
    picture: auth0User.picture,
    userID: auth0User?.sub || auth0User.email
  };

  return (
    <main>
      <Dashboard user={user} />
    </main>
  );
}

export default App;
