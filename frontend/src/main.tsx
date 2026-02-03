import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import { Auth0Provider } from "@auth0/auth0-react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const queryClient = new QueryClient({});

createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={queryClient}>
    <Auth0Provider
      domain={import.meta.env.VITE_AUTH0_DOMAIN}
      clientId={import.meta.env.VITE_AUTH0_CLIENT_ID}
      authorizationParams={{
        redirect_uri: window.location.origin,
        audience: "http://localhost:8080/api"
      }}
      cacheLocation={import.meta.env.DEV ? "localstorage" : "memory"}
    >
      <App />
    </Auth0Provider>
  </QueryClientProvider>
);
