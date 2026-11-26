import { getTranslations } from "next-intl/server";

import { LanguageControl } from "@/components/language/language-control";

import { SignInForm } from "./sign-in-form";

export default async function LoginPage() {
  const t = await getTranslations("SignIn");

  return (
    <main className="grid min-h-svh grid-rows-[auto_1fr] desktop:grid-cols-[minmax(18rem,0.9fr)_minmax(22rem,1.1fr)] desktop:grid-rows-1">
      <aside
        aria-label="Reconcilio"
        className="flex flex-col justify-between gap-[1.6rem] bg-panel px-[1.3rem] pt-5 pb-[1.6rem] text-panel-foreground desktop:gap-16 desktop:px-[max(2rem,6vw)] desktop:py-12"
      >
        <span
          className="wordmark desktop:flex-1 desktop:justify-center desktop:gap-[0.28em]
            desktop:text-[clamp(2.25rem,4.5vw,5rem)] desktop:leading-none desktop:before:size-[0.86em]
            desktop:before:shrink-0 desktop:before:rounded-[0.2em] desktop:before:border-[0.085em]"
        >
          Reconcilio
        </span>
        <div className="max-w-[27rem] border-t border-panel-divider pt-4 desktop:pt-6">
          <p className="text-[1.7rem] leading-[1.08] font-[640] tracking-[-0.04em] desktop:text-[2.6rem]">
            {t("purpose")}
          </p>
          <p className="mt-2 max-w-[40ch] text-panel-muted desktop:mt-4 desktop:max-w-[30ch]">
            {t("summary")}
          </p>
        </div>
      </aside>
      <section
        className="relative grid grid-rows-[auto_1fr] px-[1.3rem] pt-5 desktop:px-8 desktop:pt-8"
        aria-labelledby="sign-in-heading"
      >
        <div className="relative z-10 justify-self-end desktop:absolute desktop:top-8 desktop:right-8">
          <LanguageControl />
        </div>
        <div className="grid place-items-start justify-items-center pt-7 pb-10 desktop:row-span-2 desktop:place-items-center desktop:py-12">
          <div className="w-full max-w-96">
            <h1 className="page-title" id="sign-in-heading">{t("title")}</h1>
            <p className="mt-2.5 text-muted-foreground">{t("description")}</p>
            <SignInForm />
          </div>
        </div>
      </section>
    </main>
  );
}
