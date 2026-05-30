import { useState } from "react";
import { useAuth } from "@/context/AuthContext";
import { followUser, unfollowUser } from "@/api/interaction";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

interface FollowButtonProps {
  targetUserId: number;
  initialFollowing?: boolean;
}

export default function FollowButton({ targetUserId, initialFollowing = false }: FollowButtonProps) {
  const { user } = useAuth();
  const [following, setFollowing] = useState(initialFollowing);

  if (!user || user.id === targetUserId) return null;

  const handleToggle = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      if (following) {
        await unfollowUser(targetUserId);
        setFollowing(false);
      } else {
        await followUser(targetUserId);
        setFollowing(true);
      }
    } catch {
      toast.error("操作失败");
    }
  };

  return (
    <Button
      variant={following ? "secondary" : "default"}
      size="sm"
      onClick={handleToggle}
    >
      {following ? "已关注" : "关注"}
    </Button>
  );
}
