const { onDocumentUpdated } = require("firebase-functions/v2/firestore");
const { defineSecret } = require("firebase-functions/params");
const { Resend } = require("resend");

const resendApiKey = defineSecret("RESEND_API_KEY");
const resendFrom = defineSecret("RESEND_FROM_EMAIL");

exports.onWaitlistApproval = onDocumentUpdated(
  {
    document: "waitlist/{email}",
    secrets: [resendApiKey, resendFrom],
  },
  async (event) => {
    const before = event.data.before.data();
    const after = event.data.after.data();

    if (before.status === "approved" || after.status !== "approved") {
      return;
    }

    const email = after.email;
    if (!email) {
      console.warn("No email on approved waitlist doc:", event.params.email);
      return;
    }

    const appUrl = process.env.APP_URL || "https://haraesume.com";

    const html = `<div style="font-family:-apple-system,system-ui,sans-serif;max-width:480px;margin:0 auto;padding:32px 24px">
<h2 style="font-size:20px;margin:0 0 16px">You've been approved!</h2>
<p style="color:#444;line-height:1.6;margin:0 0 20px">Your Haraesume account is now active. Sign in to start tailoring your resume for every role you apply to.</p>
<a href="${appUrl}" style="display:inline-block;padding:10px 28px;background:#111;color:#fff;text-decoration:none;border-radius:4px;font-size:14px;font-weight:500">Sign in</a>
<hr style="border:none;border-top:1px solid #eee;margin:28px 0 16px">
<p style="color:#999;font-size:13px;margin:0">— Haraesume</p>
</div>`;

    const resend = new Resend(resendApiKey.value());
    const from = resendFrom.value() || "no-reply@ackberry.dev";

    try {
      await resend.emails.send({
        from,
        to: [email],
        subject: "You're in — Haraesume",
        html,
      });
      console.log("Approval email sent to", email);
    } catch (err) {
      console.error("Failed to send approval email to", email, err);
    }
  }
);
