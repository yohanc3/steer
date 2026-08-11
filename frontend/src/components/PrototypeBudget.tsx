import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";

type Category = { name: string; monthly_limit_cents: number };
type Budget = { monthly_total_cents: number; categories: Category[]; version: number; updated_at: string };
type Transaction = { id: string; merchant: string; provider_category: string; amount_cents: number; occurred_at: string; budget_version: number; budget_category?: string; classification_status: "matched" | "unmatched"; classification_source: "ai" | "cache" | "unmatched" | "user" };
type Diagnostics = { storage: string; ai_model: string; ai_configured: boolean; mapping_scope: string };
type State = { budget: Budget | null; transactions: Transaction[]; diagnostics: Diagnostics };

const emptyState: State = { budget: null, transactions: [], diagnostics: { storage: "SQLite", ai_model: "", ai_configured: false, mapping_scope: "" } };
const money = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/prototype-budget${path}`, { headers: { "Content-Type": "application/json", ...options?.headers }, ...options });
  const text = await response.text();
  let payload: { error?: string; result?: { reason?: string } } & T;
  try {
    payload = JSON.parse(text) as { error?: string; result?: { reason?: string } } & T;
  } catch {
    throw new Error(`HTTP ${response.status}: ${text || response.statusText}`);
  }
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${payload.error ?? payload.result?.reason ?? "request failed without an error payload"}`);
  return payload as T;
}

export default function PrototypeBudget() {
  const [state, setState] = useState(emptyState);
  const [budgetInput, setBudgetInput] = useState("");
  const [merchant, setMerchant] = useState("");
  const [providerCategory, setProviderCategory] = useState("");
  const [amount, setAmount] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<Record<string, string>>({});
  const [trace, setTrace] = useState<string[]>(["GET /api/prototype-budget → waiting for prototype state"]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = async () => setState(await api<State>(""));
  useEffect(() => { refresh().then(() => setTrace(["GET /api/prototype-budget → SQLite state loaded", "Browser session cookie scopes this prototype data"])).catch((reason: Error) => setError(reason.message)); }, []);

  const categorySpend = useMemo(() => {
    const totals: Record<string, number> = {};
    for (const transaction of state.transactions) if (transaction.budget_version === state.budget?.version && transaction.budget_category) totals[transaction.budget_category] = (totals[transaction.budget_category] ?? 0) + transaction.amount_cents;
    return totals;
  }, [state]);

  async function submitBudget(event: FormEvent) {
    event.preventDefault();
    if (!budgetInput.trim()) return;
    setBusy(true); setError("");
    try {
      const response = await api<{ result: { status: "valid" | "invalid"; reason?: string }; state: State }>("/chat", { method: "POST", body: JSON.stringify({ message: budgetInput }) });
      setState(response.state);
      setTrace(response.result.status === "valid" ? ["POST /chat → DeepSeek JSON response received", "Go validation → totals, positive cents, unique category names: passed", `SQLite → budget version ${response.state.budget?.version} saved; old mapping cache is ignored`] : ["POST /chat → DeepSeek reported incomplete input", `No SQLite budget write → ${response.result.reason}`]);
      if (response.result.status === "valid") setBudgetInput("");
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Budget request failed."); }
    finally { setBusy(false); }
  }

  async function addTransaction(event: FormEvent) {
    event.preventDefault();
    const cents = Math.round(Number(amount) * 100);
    if (!merchant.trim() || !providerCategory.trim() || !Number.isFinite(cents) || cents <= 0) { setError("Merchant, Teller category, and a positive amount are required."); return; }
    setBusy(true); setError("");
    try {
      const transaction = await api<Transaction>("/transactions", { method: "POST", body: JSON.stringify({ merchant, provider_category: providerCategory, amount_cents: cents }) });
      await refresh(); setMerchant(""); setProviderCategory(""); setAmount("");
      const path = transaction.classification_source === "cache" ? "cache hit → no model call" : transaction.classification_source === "ai" ? "cache miss → DeepSeek chose an existing category → mapping cached" : "cache miss → DeepSeek returned unmatched → awaiting user decision";
      setTrace(["POST /transactions → transaction persisted", path, `result → ${transaction.classification_status}${transaction.budget_category ? ` (${transaction.budget_category})` : ""}`]);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Transaction request failed."); }
    finally { setBusy(false); }
  }

  async function resolve(transaction: Transaction) {
    const category = selectedCategories[transaction.id];
    if (!category) { setError("Select an existing category to allocate this transaction."); return; }
    setBusy(true); setError("");
    try {
      setState(await api<State>(`/unmatched/${transaction.id}/resolve`, { method: "POST", body: JSON.stringify({ category, remember: true }) }));
      setTrace(["POST /unmatched/:id/resolve → user allocation saved", `mapping cache → ${transaction.merchant} / ${transaction.provider_category} will map to ${category} for this budget version`]);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Resolution failed."); }
    finally { setBusy(false); }
  }

  async function removeBudget() {
    if (!window.confirm("Delete the budget and clear classifications?")) return;
    setBusy(true); try { await api("/budget", { method: "DELETE" }); await refresh(); setTrace(["DELETE /budget → budget, mappings, and classifications cleared", "Activity remains available to test a replacement budget"]); } catch (reason) { setError(reason instanceof Error ? reason.message : "Delete failed."); } finally { setBusy(false); }
  }
  async function reset() {
    if (!window.confirm("Delete all prototype data for this browser session?")) return;
    setBusy(true); try { await api("", { method: "DELETE" }); setState(emptyState); setTrace(["DELETE / → browser-scoped prototype data removed"]); } catch (reason) { setError(reason instanceof Error ? reason.message : "Reset failed."); } finally { setBusy(false); }
  }

  return <main className="prototype-console">
    <header><div><p>STEER / DEVELOPMENT PROTOTYPE</p><h1>Budget classification workbench</h1></div><div className="header-buttons"><button onClick={removeBudget} disabled={!state.budget || busy}>delete budget</button><button onClick={reset} disabled={busy}>reset session</button></div></header>
    <section className="system-grid"><Status label="API" value="same-origin /api" okay /><Status label="storage" value={state.diagnostics.storage} okay /><Status label="model" value={state.diagnostics.ai_model || "not configured"} okay={state.diagnostics.ai_configured} /><Status label="mapping key" value={state.diagnostics.mapping_scope || "loading"} okay /></section>
    <section className="trace"><strong>LAST EXECUTION TRACE</strong>{trace.map((entry, index) => <code key={`${entry}-${index}`}>{String(index + 1).padStart(2, "0")}  {entry}</code>)}</section>
    {error && <p className="console-error">ERROR: {error}</p>}
    <div className="console-grid">
      <section className="console-panel"><h2>1. Natural-language budget input</h2><p className="hint">DeepSeek must return either valid structured JSON or a concise insufficiency reason. Go performs the final validation and write.</p><form onSubmit={submitBudget}><textarea value={budgetInput} onChange={(event) => setBudgetInput(event.target.value)} placeholder="Monthly budget 1700 USD: 800 rent, 300 gas, 200 groceries, 400 other expenses." rows={5} /><div className="row"><button type="button" onClick={() => setBudgetInput("Monthly budget 1700 USD: 800 rent, 300 gas, 200 groceries, 400 other expenses.")}>load valid example</button><button type="button" onClick={() => setBudgetInput("Monthly budget 1500 USD: 1000 rent and 100 food.")}>load incomplete example</button><button className="primary" disabled={busy}>{busy ? "running…" : "submit to parser"}</button></div></form></section>
      <section className="console-panel"><h2>2. Current persisted budget</h2>{state.budget ? <><dl><dt>monthly total</dt><dd>{money.format(state.budget.monthly_total_cents / 100)}</dd><dt>budget version</dt><dd>{state.budget.version}</dd><dt>updated</dt><dd>{new Date(state.budget.updated_at).toLocaleString()}</dd></dl><table><thead><tr><th>category</th><th>limit</th><th>current-version spend</th></tr></thead><tbody>{state.budget.categories.map((category) => <tr key={category.name}><td>{category.name}</td><td>{money.format(category.monthly_limit_cents / 100)}</td><td>{money.format((categorySpend[category.name] ?? 0) / 100)}</td></tr>)}</tbody></table></> : <p className="hint">No budget stored for this browser session.</p>}</section>
      <section className="console-panel"><h2>3. Simulate Teller activity</h2><p className="hint">This uses the same inputs the future sync path will supply: merchant, provider category, and amount.</p><form className="transaction-form" onSubmit={addTransaction}><input value={merchant} onChange={(event) => setMerchant(event.target.value)} placeholder="merchant (Whole Foods)" /><input value={providerCategory} onChange={(event) => setProviderCategory(event.target.value)} placeholder="Teller category (Groceries)" /><input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" placeholder="amount (84.20)" /><button className="primary" disabled={!state.budget || busy}>run classification</button></form></section>
      <section className="console-panel"><h2>4. Transaction / mapping results</h2>{state.transactions.length === 0 ? <p className="hint">No activity submitted.</p> : <table><thead><tr><th>merchant</th><th>Teller category</th><th>amount</th><th>result</th><th>path</th></tr></thead><tbody>{state.transactions.map((transaction) => <tr key={transaction.id}><td>{transaction.merchant}</td><td>{transaction.provider_category}</td><td>{money.format(transaction.amount_cents / 100)}</td><td>{transaction.budget_category ?? "unmatched"}</td><td><code>{transaction.classification_source}</code></td></tr>)}</tbody></table>}</section>
    </div>
    {state.transactions.filter((transaction) => transaction.classification_status === "unmatched").map((transaction) => <section className="unmatched-console" key={transaction.id}><h2>Unmatched: {transaction.merchant} / {transaction.provider_category}</h2><p>Choose an existing category to allocate and cache this decision, or change the budget above to introduce a new category.</p><select value={selectedCategories[transaction.id] ?? ""} onChange={(event) => setSelectedCategories((current) => ({ ...current, [transaction.id]: event.target.value }))}><option value="">select current budget category</option>{state.budget?.categories.map((category) => <option key={category.name} value={category.name}>{category.name}</option>)}</select><button className="primary" onClick={() => resolve(transaction)} disabled={busy}>allocate + cache mapping</button><button onClick={() => setBudgetInput("Update my budget to add a new category for ")}>modify budget instead</button></section>)}
  </main>;
}

function Status({ label, value, okay }: { label: string; value: string; okay?: boolean }) { return <div><span className={okay ? "dot okay" : "dot"} /><strong>{label}</strong><code>{value}</code></div>; }
