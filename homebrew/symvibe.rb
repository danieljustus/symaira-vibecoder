# Homebrew formula for symvibe.
#
# For `brew install symaira/tap/symvibe` to work, this file must live in the
# `symaira/homebrew-tap` repository under `Formula/symvibe.rb`. The copy here is
# the source of truth; update the url/sha256 on every release.
class Symvibe < Formula
  desc "Vibe Coding Baukasten — a graphical cycle board that drives coding agents"
  homepage "https://github.com/danieljustus/symaira-vibecoder"
  url "https://github.com/danieljustus/symaira-vibecoder/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "PLACEHOLDER_UPDATE_ON_RELEASE"
  license "MIT"

  depends_on "go" => :build
  depends_on "git"
  depends_on "gh" => :optional

  def install
    system "make", "build"
    bin.install "symvibe"
  end

  test do
    system "#{bin}/symvibe", "version"
  end
end
