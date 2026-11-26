import { SignInForm } from "./sign-in-form";

export default function LoginPage() {
  return (
    <main className="sign-in">
      <aside className="introduction" aria-label="Reconcilio">
        <span className="wordmark">Reconcilio</span>
        <div className="introduction-copy">
          <p className="introduction-title">Reconcile transactions</p>
          <p>Compare internal records with bank and card activity.</p>
        </div>
      </aside>
      <section className="sign-in-section" aria-labelledby="sign-in-heading">
        <div className="sign-in-content">
          <h1 id="sign-in-heading">Sign in</h1>
          <p className="section-description">Use your local account to open the workspace.</p>
          <SignInForm />
        </div>
      </section>
    </main>
  );
}
