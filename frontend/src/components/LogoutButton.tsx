import { useAuth0 } from "@auth0/auth0-react";

export default function LogoutButton() {
  const { logout } = useAuth0();
  return (
    <button
      onClick={() => {
        localStorage.removeItem("authStatus");
        logout({ logoutParams: { returnTo: window.location.origin } });
      }}
      className="button logout"
    >
      Log Out
    </button>
  );
}
