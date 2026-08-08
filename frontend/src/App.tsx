import ConnectBank from "./components/ConnectBank";

export default function App() {
  const session = new URLSearchParams(window.location.search).get("session");
  if (!session) return <main>Open this page from Telegram to connect your bank.</main>;
  return <main><ConnectBank sessionToken={session} /></main>;
}
