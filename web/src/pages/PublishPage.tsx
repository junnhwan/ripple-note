import { useState, useCallback, type DragEvent, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { publishNote } from "@/api/notes";
import { uploadImage } from "@/api/upload";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ImagePlus, X, Upload, Send } from "lucide-react";
import { toast } from "sonner";
import { ApiError } from "@/api/client";

export default function PublishPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [imageUrls, setImageUrls] = useState<string[]>([]);
  const [uploading, setUploading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const handleImageUpload = useCallback(async (files: FileList) => {
    setUploading(true);
    try {
      for (const file of Array.from(files)) {
        if (!file.type.startsWith("image/")) continue;
        const result = await uploadImage(file);
        setImageUrls((prev) => [...prev, result.url]);
      }
    } catch {
      toast.error("图片上传失败");
    } finally {
      setUploading(false);
    }
  }, []);

  const handleDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer.files.length) {
        handleImageUpload(e.dataTransfer.files);
      }
    },
    [handleImageUpload]
  );

  const handleDragOver = (e: DragEvent) => e.preventDefault();

  const addTag = () => {
    const tag = tagInput.trim().toLowerCase();
    if (tag && !tags.includes(tag)) {
      setTags([...tags, tag]);
    }
    setTagInput("");
  };

  const removeTag = (tag: string) => {
    setTags(tags.filter((t) => t !== tag));
  };

  const removeImage = (url: string) => {
    setImageUrls(imageUrls.filter((u) => u !== url));
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      setError("标题不能为空");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const note = await publishNote({
        title: title.trim(),
        body: body.trim(),
        image_urls: imageUrls.length > 0 ? imageUrls : undefined,
        tags: tags.length > 0 ? tags : undefined,
      });
      toast.success("笔记发布成功，等待审核");
      navigate(`/notes/${note.id}`);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("发布失败，请重试");
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (!user) {
    return (
      <div className="page-enter flex min-h-[40vh] items-center justify-center">
        <p className="text-muted-foreground">
          请先 <a href="/login" className="text-primary hover:underline">登录</a> 后发布笔记
        </p>
      </div>
    );
  }

  return (
    <div className="page-enter mx-auto max-w-2xl">
      <Card className="border-gray-100 shadow-lg shadow-amber-500/5">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-400 shadow-md shadow-amber-200/40">
              <Send className="h-4 w-4 text-white" />
            </div>
            发布笔记
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-5">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {/* Image upload area */}
            <div className="space-y-2">
              <Label>图片</Label>
              <div
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                className="flex min-h-[120px] cursor-pointer flex-wrap items-center justify-center gap-3 rounded-xl border-2 border-dashed border-gray-200 bg-amber-50/30 p-4 transition-all duration-300 hover:border-amber-300 hover:bg-amber-50/50"
                onClick={() => {
                  const input = document.createElement("input");
                  input.type = "file";
                  input.multiple = true;
                  input.accept = "image/*";
                  input.onchange = (e) => {
                    const files = (e.target as HTMLInputElement).files;
                    if (files) handleImageUpload(files);
                  };
                  input.click();
                }}
              >
                {imageUrls.length > 0 ? (
                  <>
                    {imageUrls.map((url) => (
                      <div key={url} className="group relative h-24 w-24 overflow-hidden rounded-xl shadow-sm">
                        <img src={url} alt="" className="h-full w-full object-cover" />
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            removeImage(url);
                          }}
                          className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/50 text-white opacity-0 transition-all duration-200 group-hover:opacity-100 hover:bg-red-500"
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </div>
                    ))}
                    <div className="flex h-24 w-24 items-center justify-center rounded-xl border-2 border-dashed border-gray-200 transition-colors duration-200 hover:border-amber-300">
                      {uploading ? (
                        <Upload className="h-5 w-5 animate-bounce text-amber-500" />
                      ) : (
                        <ImagePlus className="h-5 w-5 text-gray-400" />
                      )}
                    </div>
                  </>
                ) : (
                  <div className="flex flex-col items-center gap-2 text-gray-400">
                    <ImagePlus className="h-8 w-8" />
                    <span className="text-sm">拖拽或点击上传图片</span>
                  </div>
                )}
              </div>
            </div>

            {/* Title */}
            <div className="space-y-2">
              <Label htmlFor="title">标题</Label>
              <Input
                id="title"
                placeholder="笔记标题"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={100}
                required
                className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
              />
              <span className="text-xs text-muted-foreground">{title.length}/100</span>
            </div>

            {/* Body */}
            <div className="space-y-2">
              <Label htmlFor="body">正文</Label>
              <Textarea
                id="body"
                placeholder="分享你的想法…"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={6}
                maxLength={5000}
                className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
              />
              <span className="text-xs text-muted-foreground">{body.length}/5000</span>
            </div>

            {/* Tags */}
            <div className="space-y-2">
              <Label>标签</Label>
              <div className="flex gap-2">
                <Input
                  placeholder="输入标签，回车添加"
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addTag();
                    }
                  }}
                  className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
                />
                <Button type="button" variant="secondary" size="sm" onClick={addTag}>
                  添加
                </Button>
              </div>
              {tags.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {tags.map((tag) => (
                    <span key={tag} className="tag-chip gap-1">
                      #{tag}
                      <button type="button" onClick={() => removeTag(tag)} className="ml-1 hover:text-red-500 transition-colors duration-200">
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            {/* Submit */}
            <Button
              type="submit"
              className="btn-press w-full gap-2 bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50 transition-all duration-200 hover:shadow-lg hover:shadow-amber-200/60 hover:brightness-105"
              disabled={submitting || uploading}
            >
              <Send className="h-4 w-4" />
              {submitting ? "发布中…" : "发布笔记"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
