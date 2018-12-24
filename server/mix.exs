defmodule OAAS.MixProject do
  use Mix.Project

  def project do
    [
      app: :oaas,
      version: "0.1.0",
      elixir: "~> 1.7",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  def application do
    [
      extra_applications: [:logger],
      mod: {OAAS.Application, []}
    ]
  end

  defp deps do
    [
      {:cowboy, "~> 2.6"},
      {:export, "~> 0.1"},
      {:httpoison, "~> 1.5", override: true},
      {:jason, "~> 1.1"},
      {:nostrum, git: "https://github.com/Kraigie/nostrum.git"},
      {:osu_ex, git: "https://github.com/christopher-dG/osu-ex.git"},
      {:plug_cowboy, "~> 2.0"},
      {:sqlitex, "~> 1.4"},
      {:table_rex, "~> 2.0.0"}
    ]
  end
end
