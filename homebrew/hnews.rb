class Hnews < Formula
  desc "A minimalist, premium Hacker News client"
  homepage "https://github.com/offbeatengineer/hnews"
  url "https://github.com/offbeatengineer/hnews/releases/download/v0.1.0/hnews-0.1.0.tar.gz"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/offbeatengineer/hnews/releases/download/v0.1.0/hnews-0.1.0-arm64.tar.gz"
    else
      url "https://github.com/offbeatengineer/hnews/releases/download/v0.1.0/hnews-0.1.0-amd64.tar.gz"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/offbeatengineer/hnews/releases/download/v0.1.0/hnews-0.1.0-arm64.tar.gz"
    else
      url "https://github.com/offbeatengineer/hnews/releases/download/v0.1.0/hnews-0.1.0-amd64.tar.gz"
    end
  end

  def install
    bin.install "hnews"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/hnews --version 2>&1", 1)
  end
end
