export const enUS = {
  appName: "TalentPilot",
  foundation: {
    mainLabel: "TalentPilot foundation",
    eyebrow: "Computing Power Business Unit",
    title: "TalentPilot",
    summary: "The recruiting assistant foundation is ready.",
    primaryAction: "Enter workspace",
  },
  session: {
    login: {
      mainLabel: "Login",
      title: "W3 Login",
      accountLabel: "Company account",
      passwordLabel: "Company password",
      submitAction: "Log in",
      loadingAction: "Logging in",
      checkingSession: "Loading sign-in status",
      error: "Login failed. Check your account and password, then try again.",
      successPrefix: "Signed in with W3",
    },
    nav: {
      resumeParse: "Resume parsing",
      resumeRecommend: "Resume recommendation",
    },
    workspace: {
      mainLabel: "TalentPilot workspace",
      navLabel: "Primary navigation",
      logoutAction: "Log out",
    },
  },
} as const;
