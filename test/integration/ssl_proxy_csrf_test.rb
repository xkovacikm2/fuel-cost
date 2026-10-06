require "test_helper"

# Simulates TLS terminated upstream (Cloudflare) with plain HTTP reaching the app.
class SslProxyCsrfTest < ActionDispatch::IntegrationTest
  HTTPS_ORIGIN = "https://www.example.com".freeze

  setup do
    @user = User.take
    @original_forgery_protection = ActionController::Base.allow_forgery_protection
    ActionController::Base.allow_forgery_protection = true
  end

  teardown do
    ActionController::Base.allow_forgery_protection = @original_forgery_protection
  end

  test "https origin is rejected when app sees plain http without assume_ssl" do
    post session_path, params: sign_in_params(fetch_csrf_token), headers: { "Origin" => HTTPS_ORIGIN }

    assert_response :unprocessable_content
    assert_nil cookies[:session_id]
  end

  test "https origin is accepted when assume_ssl is enabled" do
    with_assume_ssl do
      post "/session", params: sign_in_params(fetch_csrf_token), headers: { "Origin" => HTTPS_ORIGIN }

      assert_redirected_to "#{HTTPS_ORIGIN}/"
      assert cookies[:session_id]
    end
  end

  private
    def with_assume_ssl
      @app = ActionDispatch::AssumeSSL.new(Rails.application)
      reset!
      yield
    ensure
      @app = nil
    end

    def app
      @app || super
    end

    def fetch_csrf_token
      get "/session/new"
      css_select('meta[name="csrf-token"]').first["content"]
    end

    def sign_in_params(token)
      { authenticity_token: token, email_address: @user.email_address, password: "password" }
    end
end
