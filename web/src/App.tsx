import { Routes, Route } from "react-router-dom";
import Layout from "@/components/layout/Layout";
import FeedPage from "@/pages/FeedPage";
import LoginPage from "@/pages/LoginPage";
import RegisterPage from "@/pages/RegisterPage";
import PublishPage from "@/pages/PublishPage";
import NoteDetailPage from "@/pages/NoteDetailPage";
import ProfilePage from "@/pages/ProfilePage";
import AdminReviewPage from "@/pages/AdminReviewPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<FeedPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/publish" element={<PublishPage />} />
        <Route path="/notes/:noteId" element={<NoteDetailPage />} />
        <Route path="/me" element={<ProfilePage />} />
        <Route path="/admin/review" element={<AdminReviewPage />} />
      </Route>
    </Routes>
  );
}
