import ConnectBank from "./components/ConnectBank";
import PrototypeBudget from "./components/PrototypeBudget";

export default function App() {
  if (window.location.pathname === "/prototype-budget") return <PrototypeBudget />;
  const session = new URLSearchParams(window.location.search).get("session");
  const nonce = new URLSearchParams(window.location.search).get("nonce");
  if (!session || !nonce) return <main>Open this page from Telegram to connect your bank.</main>;
  return <main><ConnectBank sessionToken={session} nonce={nonce} /></main>;
}
