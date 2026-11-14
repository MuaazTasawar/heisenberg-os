import type { Metadata } from "next";
import RegisterForm from "./RegisterForm";

export const metadata: Metadata = { title: "New identity" };

export default function RegisterPage() {
  return <RegisterForm />;
}
