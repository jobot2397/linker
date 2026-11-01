# Homebrew formula for linker.
#
# Install from this repo as a tap:
#   brew tap jobot2397/linker https://github.com/jobot2397/linker
#   brew install linker
#
# Or HEAD (latest main):
#   brew install --HEAD linker
#
# After cutting a release, refresh version + revision:
#   ./scripts/update-homebrew-formula.sh

class Linker < Formula
  desc "Manage dotfile symlinks from linker.toml (CLI + TUI)"
  homepage "https://github.com/jobot2397/linker"
  url "https://github.com/jobot2397/linker.git",
      tag:      "v0.1.0",
      revision: "0725bd462a5fd2f9d50e13dd223f8efb49c3f19c"
  version "0.1.0"
  license "MIT"
  head "https://github.com/jobot2397/linker.git", branch: "main"

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/linker"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/linker version")
    assert_match "Usage:", shell_output("#{bin}/linker help")
  end
end
