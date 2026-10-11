import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import { Auth0Provider } from "@auth0/auth0-react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const queryClient = new QueryClient({});
const auth0Audience = import.meta.env.VITE_AUTH0_AUDIENCE;

createRoot(document.getElementById("root")!).render(
  <Auth0Provider
    domain={import.meta.env.VITE_AUTH0_DOMAIN}
    clientId={import.meta.env.VITE_AUTH0_CLIENT_ID}
    authorizationParams={{
      redirect_uri: window.location.origin,
      audience: auth0Audience,
      scope: "openid profile email offline_access",
    }}
    cacheLocation={import.meta.env.DEV ? "localstorage" : "memory"}
    useRefreshTokens={true}
    useRefreshTokensFallback={true}
  >
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </Auth0Provider>
);
