# Homebrew formula template for Veil
#
# The release workflow renders this template with real SHA256 values and can publish the
# generated file to PAIArtCom/homebrew-veil/Formula/veil.rb.
#
# Generate locally with:
#   scripts/gen-homebrew-formula.sh <version>

class Veil < Formula
  desc "Local de-identification proxy for AI coding agents"
  homepage "https://veil.sh"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/PAIArtCom/Veil/releases/download/v#{version}/veil-v#{version}-darwin-arm64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_ARM64"
    end

    on_intel do
      url "https://github.com/PAIArtCom/Veil/releases/download/v#{version}/veil-v#{version}-darwin-amd64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/PAIArtCom/Veil/releases/download/v#{version}/veil-v#{version}-linux-arm64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_ARM64"
    end

    on_intel do
      url "https://github.com/PAIArtCom/Veil/releases/download/v#{version}/veil-v#{version}-linux-amd64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64"
    end
  end

  def install
    bin.install "veil"
  end

  test do
    assert_match "veil", shell_output("#{bin}/veil version")
  end
end
